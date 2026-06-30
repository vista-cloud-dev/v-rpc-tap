VSLRTRPTST ; VSL RPC TAP — VSLRTRP off-path reaper tests (YDB+IRIS, bare logic)
 ; Run: m test tests/VSLRTRPTST.m --engine ydb  --docker m-test-engine --routines src --routines ../m-stdlib/src
 ;      m test tests/VSLRTRPTST.m --engine iris --docker m-test-iris   --routines src --routines ../m-stdlib/src
 ; The reaper LOGIC (lease/duration/overflow/orphan/purge/health) is engine-intrinsic +
 ; ^XTMP only -> bare-testable. The TaskMan queue/requeue is guarded ($text(^%ZTLOAD))
 ; and no-ops here; its live behaviour is proven on vehu/foia in P2.
 new pass,fail
 do start^STDASSERT(.pass,.fail)
 do tLeaseStaleDisarms(.pass,.fail)
 do tLeaseFreshStaysArmed(.pass,.fail)
 do tDurationExpiredDisarms(.pass,.fail)
 do tOverflowDisarms(.pass,.fail)
 do tOrphanReapsDeadJob(.pass,.fail)
 do tOrphanKeepsLiveJob(.pass,.fail)
 do tPurgeRefreshesNode(.pass,.fail)
 do tHealthPublishes(.pass,.fail)
 do tRunCyclesAndNoOpRequeueOnBare(.pass,.fail)
 do tWatchdogRestartsStaleReaper(.pass,.fail)
 do tWatchdogNoopFreshReaper(.pass,.fail)
 do tWatchdogNoopWhenDisarmed(.pass,.fail)
 do tWatchdogRestartsOnMissingHeartbeat(.pass,.fail)
 do tPastHelper(.pass,.fail)
 do report^STDASSERT(pass,fail)
 quit
 ;
past(secs) ; a $H value <secs> seconds from now (negative = in the past)
 new d,s
 set d=$piece($horolog,",",1),s=$piece($horolog,",",2)+secs
 for  quit:s<86400  set s=s-86400,d=d+1
 for  quit:s'<0  set s=s+86400,d=d-1
 quit d_","_s
 ;
tLeaseStaleDisarms(pass,fail) ;@TEST "stale host lease -> reaper disarms (dead-man)"
 kill ^XTMP("VSLRT") set ^XTMP("VSLRT","ON")=1,^XTMP("VSLRT","LEASE")=$$past(-120)
 do cycle^VSLRTRP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"disarmed on stale lease")
 quit
 ;
tLeaseFreshStaysArmed(pass,fail) ;@TEST "fresh host lease -> stays armed"
 kill ^XTMP("VSLRT") set ^XTMP("VSLRT","ON")=1,^XTMP("VSLRT","LEASE")=$$past(120)
 do cycle^VSLRTRP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","ON")),1,"still armed on fresh lease")
 quit
 ;
tDurationExpiredDisarms(pass,fail) ;@TEST "expired duration cap -> reaper disarms (C3a backstop)"
 kill ^XTMP("VSLRT") set ^XTMP("VSLRT","ON")=1,^XTMP("VSLRT","EXP")=$$past(-1)
 do cycle^VSLRTRP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"disarmed past expiry")
 quit
 ;
tOverflowDisarms(pass,fail) ;@TEST "ring depth over cap AND rising -> overflow disarm + flag"
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","ON")=1,^XTMP("VSLRT","LEASE")=$$past(120),^XTMP("VSLRT","OVCAP")=10
 set ^XTMP("VSLRT","buf",100,"seq")=50,^XTMP("VSLRT","buf",100,"head")=1
 set ^XTMP("VSLRT","health","lastdepth")=5
 do overflow^VSLRTRP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"disarmed on overflow")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","health","overflow")),1,"overflow flagged")
 quit
 ;
tOrphanReapsDeadJob(pass,fail) ;@TEST "ring for a non-live $J is reaped (C2; per-engine liveness U3/R24)"
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","buf",999999,"seq")=2,^XTMP("VSLRT","buf",999999,"head")=1
 do orphans^VSLRTRP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",999999)),"dead-$J ring reaped")
 quit
 ;
tOrphanKeepsLiveJob(pass,fail) ;@TEST "ring for THIS live process is kept"
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","buf",$job,"seq")=2,^XTMP("VSLRT","buf",$job,"head")=1
 do orphans^VSLRTRP()
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","buf",$job)),"live-$J ring kept")
 quit
 ;
tPurgeRefreshesNode(pass,fail) ;@TEST "purge keeps ^XTMP(\"VSLRT\",0) ahead of today (A9)"
 kill ^XTMP("VSLRT")
 do purge^VSLRTRP()
 do true^STDASSERT(.pass,.fail,$piece($get(^XTMP("VSLRT",0)),"^",1)>$piece($horolog,",",1),"purge date in the future")
 quit
 ;
tHealthPublishes(pass,fail) ;@TEST "health publishes armed-state + depth for doctor"
 kill ^XTMP("VSLRT") set ^XTMP("VSLRT","ON")=2
 do health^VSLRTRP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","health","on")),2,"health on=2")
 quit
 ;
tRunCyclesAndNoOpRequeueOnBare(pass,fail) ;@TEST "run() does a cycle; requeue no-ops on a bare engine (no fault)"
 kill ^XTMP("VSLRT") set ^XTMP("VSLRT","ON")=1,^XTMP("VSLRT","LEASE")=$$past(120)
 do run^VSLRTRP()
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","health","on")),"cycle ran (health published)")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","ON")),1,"still armed (fresh lease, no TaskMan fault)")
 quit
 ;
tWatchdogRestartsStaleReaper(pass,fail) ;@TEST "C3b/#19: armed + stale reaper heartbeat -> watchdog restarts (re-queue attempted, counter bumped)"
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","ON")=1,^XTMP("VSLRT","health","at")=$$past(-120) ; reaper last beat 2 min ago
 do watchdog^VSLRTRP()
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","health","wdrestart")),1,"watchdog logged one restart on a dead reaper")
 quit
 ;
tWatchdogNoopFreshReaper(pass,fail) ;@TEST "C3b/#19: armed + fresh heartbeat -> watchdog is a no-op (reaper alive)"
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","ON")=1,^XTMP("VSLRT","health","at")=$horolog ; just beat
 do watchdog^VSLRTRP()
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","health","wdrestart")),0,"no restart while the reaper is alive")
 quit
 ;
tWatchdogNoopWhenDisarmed(pass,fail) ;@TEST "C3b/#19: disarmed -> watchdog never restarts (nothing to guard)"
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","health","at")=$$past(-120) ; stale, but capture is OFF
 do watchdog^VSLRTRP()
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","health","wdrestart")),0,"no restart when disarmed")
 quit
 ;
tWatchdogRestartsOnMissingHeartbeat(pass,fail) ;@TEST "C3b/#19: armed + never-published heartbeat -> treated as dead, restart"
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","ON")=1 ; armed but health/at never written (reaper never started)
 do watchdog^VSLRTRP()
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","health","wdrestart")),1,"missing heartbeat -> restart")
 quit
 ;
tPastHelper(pass,fail) ;@TEST "$$past compares $H correctly"
 do true^STDASSERT(.pass,.fail,$$past^VSLRTRP($$past(-5)),"a past $H is past")
 do true^STDASSERT(.pass,.fail,'$$past^VSLRTRP($$past(120)),"a future $H is not past")
 quit
