package main

import (
    "fmt"
    "os"
    "strings"
    "encoding/json"
    "io/ioutil"
    // "github.com/tidwall/gjson"
    "github.com/google/uuid"
)

type HealthMonitor struct {
    Id                 string
    Type               string
    Delay              int
    ExpectedCodes      string  `json:"expected_codes"`
    MaxRetries         int        `json:"max_retries"`
    HttpMethod         string    `json:"http_method"`
    Timeout            int
    UrlPath            string    `json:"url_path"`
}

type Member struct {
    Id                 string
    Address            string
    ProtocolPort       int16      `json:"protocol_port"`
}

type Pool struct {
    Id                 string
    Name               string
    HealthmonitorId    string     `json:"healthmonitor_id"`
    Members            []Member
    LbAlgorithm        string  `json:"lb_algorithm"`
}

type Listener struct {
    Id                 string
    Name               string
    Protocol           string
    ProtocolPort       int16      `json:"protocol_port"`
    DefaultPoolId      string     `json:"default_pool_id"`
}

type LoadBalancer struct {
    Id                 string
    Name               string
    Pools              []Pool
    Listeners          []Listener
    Description        string
    VipAddress         string     `json:"vip_address"`
}

type LBBundle struct {
    Loadbalancer       LoadBalancer
    Listeners          []Listener
    Pools              []Pool
    Members            []Member
    Healthmonitors     []HealthMonitor
}

func NoDash(uuid string) string {
    return strings.ReplaceAll(uuid, "-", "_")
}

func ConstructTenant(lbbundle LBBundle) interface{} {
    as3body := map[string]interface{}{
        "class": "AS3",
        "action": "deploy",
        "declaration": map[string]interface{}{
            "class": "ADC",
            "schemaVersion": "3.19.0",
            "updateMode": "selective",
            "id": fmt.Sprintf("urn:uuid:%s", uuid.New().String()),
            fmt.Sprintf("T_%s", NoDash(lbbundle.Loadbalancer.Id)): map[string]interface{}{
                "class": "Tenant",
                fmt.Sprintf("A_%s", NoDash(lbbundle.Loadbalancer.Id)): ConstructApplication(lbbundle),
            },
        },
    }
    return as3body
}

func ConstructApplication(lbbundle LBBundle) interface{} {
    app := map[string]interface{}{
        "class": "Application",
        "template": "generic",
    }

    ConstructService(lbbundle, app)

    return app
}

func GetPool(lbbundle LBBundle, id string) (Pool, error) {
    for _, pl := range lbbundle.Pools {
        if pl.Id == id {
            return pl, nil
        }
    }
    return Pool{}, fmt.Errorf("Pool %s not found!", id)
}

func GetMember(lbbundle LBBundle, id string) (Member, error) {
    for _, mb := range lbbundle.Members {
        if mb.Id == id {
            return mb, nil
        }
    }
    return Member{}, fmt.Errorf("Member %s not found!", id)
}

func GetListener(lbbundle LBBundle, id string) (Listener, error) {
    for _, ls := range lbbundle.Listeners {
        if ls.Id == id {
            return ls, nil
        }
    }
    return Listener{}, fmt.Errorf("Listener %s not found!", id)
}

func GetHealthMonitor(lbbundle LBBundle, id string) (HealthMonitor, error) {
    for _, hm := range lbbundle.Healthmonitors {
        if hm.Id == id {
            return hm, nil
        }
    }
    return HealthMonitor{}, fmt.Errorf("Health Monitor %s not found!", id)
}

func ConstructService(lbbundle LBBundle, app map[string]interface{}) {

    for _, ls := range lbbundle.Loadbalancer.Listeners {
        found, err := GetListener(lbbundle, ls.Id)
        if err != nil {
            panic(fmt.Errorf("Invalid LoadBalancer bundle: %s", err.Error()))
        }
        
        servName := fmt.Sprintf("S_%s", NoDash(found.Id))
        serv := map[string]interface{}{}
        if found.Protocol == "HTTP" {
            serv["class"] = "Service_HTTP"
            
        } else if found.Protocol == "TCP" {
            serv["class"] = "Service_TCP"
            serv["virtualPort"] = found.ProtocolPort
        }

        serv["virtualAddresses"] = []string{
            lbbundle.Loadbalancer.VipAddress,
        }

        if found.DefaultPoolId != "" {
            serv["pool"] = fmt.Sprintf("P_%s", NoDash(found.DefaultPoolId))
            ConstructPool(lbbundle, found.DefaultPoolId, app)
        }

        app[servName] = serv
    }
}

func ConstructPool(
    lbbundle LBBundle, poolId string, app map[string]interface{},
) {
    found, err := GetPool(lbbundle, poolId)
    if err != nil {
        panic(fmt.Errorf("Invalid LB Bundle: %s", err.Error()))
    }

    poolName := fmt.Sprintf("P_%s", NoDash(found.Id))
    hmName := fmt.Sprintf("HM_%s", NoDash(found.HealthmonitorId))
    pool := map[string]interface{} {
        "class": "Pool",
        "monitors": []map[string]interface{}{
            {"use": hmName},
        },
        "loadBalancingMode": "round-robin",
    }
    if found.LbAlgorithm == "LEAST_CONNECTIONS" {
        pool["loadBalancingMode"] = "least-connections-member"
    } else if found.LbAlgorithm == "xxxxxxxx" {

    }

    ConstructHealthMonitor(lbbundle, found.HealthmonitorId, app)

    members := []map[string]interface{}{}
    for _, mb := range found.Members {
        m := ConstructMember(lbbundle, mb.Id)
        members = append(members, m)
    }
    pool["members"] = members
    app[poolName] = pool
}

func ConstructMember(lbbundle LBBundle, id string) map[string]interface{} {
    found, err := GetMember(lbbundle, id)
    if err != nil {
        panic(fmt.Errorf("Invalid LB Bundle: %s", err.Error()))
    }
    m := map[string]interface{} {
        "servicePort": found.ProtocolPort,
        "serverAddresses": []string{
            found.Address,
        },
    }
    return m
}

func ConstructHealthMonitor(
    lbbundle LBBundle, hmId string, app map[string]interface{},
) {
    found, err := GetHealthMonitor(lbbundle, hmId)
    if err != nil {
        panic(fmt.Errorf("Invalid LB Bundle: %s", err.Error()))
    }

    hm := map[string]interface{}{
        "class": "Monitor",
        "timeout": found.Timeout,
        "interval": found.Delay,
    }
    if found.Type == "PING" {
        hm["monitorType"] = "icmp"
    } else if found.Type == "HTTP" {
        hm["monitorType"] = "http"

    } else if found.Type == "TCP" {
        hm["monitorType"] = "tcp"
    }

    hmName := fmt.Sprintf("HM_%s", NoDash(hmId))
    app[hmName] = hm
}

func main() {
    if len(os.Args) != 2 {
        fmt.Printf("%s %s\n", os.Args[0], "<file path>")
        return
    }

    filePath := os.Args[1]
    lbbundleBytes, err := ioutil.ReadFile(filePath)
    if err != nil {
        panic(fmt.Errorf("Failed to read file: %s", err.Error()))
    }
    var lbbundle LBBundle
    err = json.Unmarshal(lbbundleBytes, &lbbundle)
    if err != nil {
        panic(fmt.Errorf("failed to unmarshal loadbalancer body: %s", err.Error()))
    }

    decl := ConstructTenant(lbbundle)
    d, err := json.Marshal(decl)
    if err != nil {
        fmt.Println("failed to marshal")
        return
    }

    fmt.Printf("%s\n", string(d))
}