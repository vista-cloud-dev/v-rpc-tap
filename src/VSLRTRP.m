VSLRTRP ; VSL RPC TAP — off-path reaper: enforce every disable condition off the hot path.
 ; doc: @exrun bare
 ; doc: @exsafe transactional
 ;
 ; The single off-path engine actor (TaskMan, ~10 s). It enforces the conditions kept
 ; OUT of the in-path tap (D9): host-lease dead-man, absolute duration cap, ring-overflow
 ; disarm, orphan dead-$J reaping, purge-node upkeep, health publish. References NO
 ; STD*/VSL* routine (D10); self-contained ($$past is local, not borrowed from VSLRTAP).
 ; Its ONLY VistA tie is Kernel TaskMan (REQ^%ZTLOAD) for self-scheduling — guarded by
 ; $text(^%ZTLOAD) so the routine is bare-safe; the live requeue is proven on vehu/foia (P2).
 ;
 ; If the reaper dies, storage is still safe (in-path drop-oldest), traffic is still safe
 ; (observe-only), and the duration cap still fires IN-PATH (C3a). Only lease/overflow
 ; auto-disable latency degrades until the #19 watchdog (C3b) restarts it.
 quit
 ;
start() ; queue the reaper the first time (host calls this after arm). No-op on a bare engine.
 new ZTDTH,ZTIO,ZTRTN,ZTSK
 quit:$text(+0^%ZTLOAD)=""
 set ZTRTN="run^VSLRTRP",ZTDTH=$$next(),ZTIO=""
 do ^%ZTLOAD
 quit
 ;
run() ; TaskMan entry: one reaper pass, then re-queue self while still armed
 do cycle()
 if $get(^XTMP("VSLRT","ON")) do requeue()
 quit
 ;
requeue() ; re-queue self ~10 s out via Kernel TaskMan. NO-OP on a bare engine (no %ZTLOAD).
 new ZTDTH,ZTIO,ZTRTN,ZTSK
 quit:$text(+0^%ZTLOAD)=""
 set ZTRTN="run^VSLRTRP",ZTDTH=$$next(),ZTIO=""
 do REQ^%ZTLOAD
 quit
 ;
watchdog() ; #19 OPTION watchdog (C3b): restart the reaper if it died while still armed.
 ; Runs from its OWN TaskMan-scheduled option (file #19), at a slower cadence than and
 ; INDEPENDENT of the reaper itself — so it survives the reaper's death (the reaper
 ; cannot guard its own liveness). Death = the reaper heartbeat
 ; ^XTMP("VSLRT","health","at") (set every cycle by health()) gone stale, or never
 ; published, while capture is still armed. The actual #19 OPTION + its TaskMan
 ; schedule ship in the KIDS build; this is the entry point it calls.
 new at
 quit:'$get(^XTMP("VSLRT","ON"))  ; not armed -> nothing to guard
 set at=$get(^XTMP("VSLRT","health","at"))
 quit:at]""&'$$stale(at)  ; fresh heartbeat -> reaper alive, leave it be
 set ^XTMP("VSLRT","health","wdrestart")=1+$get(^XTMP("VSLRT","health","wdrestart"))
 do start()  ; re-queue the reaper (no-op on a bare engine; live via TaskMan)
 quit
 ;
stale(at) ; 1 if the reaper heartbeat <at> ($H) is older than the watchdog threshold
 new lim
 set lim=+$get(^XTMP("VSLRT","WDSEC")) if 'lim set lim=60
 quit $$past($$plus(at,lim))
 ;
cycle() ; one reaper pass — all off-path disable conditions (bare-testable)
 do lease(),duration(),overflow(),orphans(),purge(),health()
 quit
 ;
lease() ; host-lease dead-man: clear ON if the host stopped refreshing the lease
 new v
 set v=$get(^XTMP("VSLRT","LEASE"))
 if v]"",$$past(v) kill ^XTMP("VSLRT","ON")
 quit
 ;
duration() ; absolute duration cap (reaper backstop to the in-path C3a check)
 new v
 set v=$get(^XTMP("VSLRT","EXP"))
 if v]"",$$past(v) kill ^XTMP("VSLRT","ON")
 quit
 ;
overflow() ; ring-overflow disarm: aggregate depth over cap AND still rising -> host not draining
 new d
 set d=$$depth()
 if d>$$cap(),d>+$get(^XTMP("VSLRT","health","lastdepth")) kill ^XTMP("VSLRT","ON") set ^XTMP("VSLRT","health","overflow")=1
 set ^XTMP("VSLRT","health","lastdepth")=d,^XTMP("VSLRT","health","depth")=d
 quit
 ;
orphans() ; reap rings whose $J is no longer a live process (C2; per-engine liveness U3/R24)
 new alive,j,sv
 ; liveness diverges: $ZGETJPI (YDB) vs ^$JOB (IRIS), neither compiles on the other ->
 ; XECUTE the engine-correct primitive (the R12 idiom; the build can substitute a literal).
 set sv=$select(($zv["GT.M")!($zv["YottaDB"):"set alive=$ZGETJPI(j,""ISPROCALIVE"")",1:"set alive=''$data(^$JOB(j))")
 set j=""
 for  set j=$order(^XTMP("VSLRT","buf",j)) quit:j=""  do
 . set alive=1 xecute sv
 . if 'alive set ^XTMP("VSLRT","health","orphans")=1+$get(^XTMP("VSLRT","health","orphans")) kill ^XTMP("VSLRT","buf",j)
 quit
 ;
purge() ; keep ^XTMP("VSLRT",0) purge-date ahead of today so the namespace isn't purged mid-capture (A9)
 new c
 set c=$piece($get(^XTMP("VSLRT",0)),"^",2,999)
 set ^XTMP("VSLRT",0)=($piece($horolog,",",1)+7)_"^"_$select(c]"":c,1:$horolog)
 quit
 ;
health() ; publish armed-state + heartbeat for `v rpc-tap doctor`
 set ^XTMP("VSLRT","health","on")=$get(^XTMP("VSLRT","ON")),^XTMP("VSLRT","health","at")=$horolog
 quit
 ;
depth() ; aggregate live record count across all job rings
 new d,j
 set d=0,j=""
 for  set j=$order(^XTMP("VSLRT","buf",j)) quit:j=""  set d=d+($get(^XTMP("VSLRT","buf",j,"seq"))-$get(^XTMP("VSLRT","buf",j,"head"),1)+1)
 quit d
 ;
cap() ; aggregate ring-depth overflow threshold; default 100000
 new c
 set c=+$get(^XTMP("VSLRT","OVCAP"))
 quit $select(c:c,1:100000)
 ;
past(h) ; 1 if now ($H) is past the $H value <h>, else 0
 new d,nd,ns,s
 set nd=$piece($horolog,",",1),ns=$piece($horolog,",",2)
 set d=$piece(h,",",1),s=$piece(h,",",2)
 quit (nd>d)!((nd=d)&(ns>s))
 ;
next() ; $H + ~10 s (the reaper interval) for the next (re)queue
 new s
 set s=$piece($horolog,",",2)+10
 quit $piece($horolog,",",1)_","_s
 ;
plus(h,secs) ; return the $H value <h> advanced by <secs> seconds (day rollover)
 new d,s
 set d=$piece(h,",",1),s=$piece(h,",",2)+secs
 for  quit:s<86400  set s=s-86400,d=d+1
 quit d_","_s
