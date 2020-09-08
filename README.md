# lb2as3

This is a spike project(written in Golang) for translating loadbalancer objects(in json format) from OpenStack world to as3 declarations in F5 BIG-IP world.

The loadbalancer json should meet the following format requirements.

* It is a complete json.
* The json body contains the following keys
    ```
    {
        "loadbalancer": {},
        "listeners": [],
        "pools": [],
        "members": [],
        "healthmonitors": [],
        "l7policies": [],
        "l7policy_rules": [],
    }
    ```

* More detailed key-values within above `[]` should comply to the data used by lbaasv2/octavia.

## LBaaSv2/Octavia Load Balancer Object Sample

Refer to `lb_samples`.

## Notes

In real practice, lbaasv2/octavia passes the bundle of loadbalancer object **piece by piece**. Refer to [./tmpdata/octavia-create.log](./tmpdata/octavia-create.log).

The loadbalancer and its sub objects are created by steps: 

The `*` means possible multi-times.
```
   -> create_load_balancer 
   -> create_listener* 
   -> create_pool* 
   -> create_member* 
   -> create_health_monitor*
   ...
```

So if you plan to use as3 declaration for loadbalancer deployment, you should have a way to compose the above loadbalancer object instead of translating/deploying multi-times for all the pieces.

The lbaasv2/octavia will not deploy the next piece until the previous one's provision_status becomes `ACTIVE` from `PENDING_CREATE`.