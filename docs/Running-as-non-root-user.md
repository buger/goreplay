You can enable Gor for non-root users in a secure method by using the following commands

``` 
# Following commands assume that you put `gor` binary to /usr/local/bin
add gor
addgroup <username> gor
chgrp gor /usr/local/bin/gor
chmod 0750 /usr/local/bin/gor
setcap "cap_net_raw,cap_net_admin+eip" /usr/local/bin/gor
```
 
As a brief explanation of the above.
* We create a group called gor. 
* We then add the user you want to the new group so they will be able to use gor without sudo
* We then change the user/group of gor binary the new group.
* We then make sure the permissions are set on gor binary so that members of the group can execute it but other normal users cannot.
* We then use `setcap` to give the CAP_NET_RAW and CAP_NET_ADMIN privilege to the executable when it runs. This is so that Gor can open its raw socket which is not normally permitted unless you are root.