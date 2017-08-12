You can enable GoReplay for non-root users in a secure method by using the following commands

``` 
# Following commands assume that you put `goreplay` binary to /usr/local/bin
add goreplay
addgroup <username> goreplay
chgrp gor /usr/local/bin/goreplay
chmod 0750 /usr/local/bin/goreplay
setcap "cap_net_raw,cap_net_admin+eip" /usr/local/bin/goreplay
```
 
As a brief explanation of the above.
* We create a group called goreplay. 
* We then add the user you want to the new group so they will be able to use gor without sudo
* We then change the user/group of goreplay binary the new group.
* We then make sure the permissions are set on gor binary so that members of the group can execute it but other normal users cannot.
* We then use `setcap` to give the CAP_NET_RAW and CAP_NET_ADMIN privilege to the executable when it runs. This is so that GoReplay can open its raw socket which is not normally permitted unless you are root.
