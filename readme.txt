=======
ghhooks
=======

A simple server that listens to github webhook push events
and run commands/scripts as configured. 

-------
Usecase
-------
It can be used to update services/websites/other-servers
running on machine on code change, automatically

--------
Features
--------
* run builds in a queue manner to not overlap multiple builds
    when they are triggered before one build is finished to keep
    things consistent and predicitable

* toml based configuration
* supports verified push events signed with configured secret
* graceful shutdown (drains all build queue but still lets the 
    running build finish)
* status page that reports live status on last started build
    (using websockets)
* configurable step-timeout (by default timeout for individual step is 10 minutes)
* branch filtering (build will only run if code is pushed to configured branch)
* everything is saved in memory (status reports for build (only last build status is saved))


----
TODO
----

* Proxy server that sends event to all configured agents which would run builds on projects
    on machines that they are running on. (A website/service can be behind a load balancer and 
    could have more than one machine on which they are running, this feature would make it easy
    to update all of them at once without configuring more than one webhook on github repo)

* currently listener is agent based, to run a build, the agent must be running on same machine alongside
project that its supposed to update on push events, with ssh client configured, projects could be updated
without an agent being setup alongside project that are supposed to update

* ansible playbook like install scripts that run on multiple machines and run series of commands/scripts
    using ssh client.

* cancel running build.
* update commit status on github repo itself using github api (needs personal token configured by user)
