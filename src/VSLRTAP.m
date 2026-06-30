VSLRTAP ; VSL RPC TAP — in-path RPC capture (the splice body).
 ; doc: @exrun bare
 ; doc: @exsafe observe-only
 ;
 ; The entire code that runs on the live RPC dispatch path. Spliced into
 ; CALLP^XWBPRS as two BARE calls (no logic in the broker, D7):
 ;   line 15.5:  D req^VSLRTAP()   (after the denial line, before the dispatch block)
 ;   line 17.5:  D rsp^VSLRTAP()   (last dotted line inside the line-16 DO block)
 ; $IO is the LIVE client socket at BOTH points (CF4) -> emit ZERO I/O, ever.
 ;
 ; Capture is OFF unless ^XTMP("VSLRT","ON") is set (0/1/2): 1=names-only,
 ; 2=full-payload. The first error self-disables the tap (fail-open, fail^).
 ; Per-job ring at ^XTMP("VSLRT","buf",$J,seq); no shared WRITTEN node in-path
 ; (D8). References NO STD*/VSL* routine and does zero crypto/JSON/I/O (D10).
 ;
 ; Record schema v1 (value at ^XTMP("VSLRT","buf",$J,seq)):
 ;   ver ^ kind ^ $H ^ rpc-name      ($J + seq are in the subscript;
 ;                                    the per-incarnation token is the "inc" node)
 quit
 ;
req() ; splice 15.5 — request capture. $IO is the LIVE socket (CF4): emit ZERO I/O.
 new sv,zr set zr="" ; zr="" -> no prior naked ref to restore (xecute below overwrites)
 ; (0) save the naked reference FIRST, before any global reference. The naked-ref
 ; SVN name diverges ($REFERENCE on YDB, $ZREFERENCE on IRIS) and is not portably
 ; settable, so the read is XECUTE'd by engine (R12). The KIDS build substitutes a
 ; bare per-engine SVN literal here at ship time (the build token); the XECUTE is
 ; the portable, token-free fallback proven green on both engines.
 set sv=$select(($zv["GT.M")!($zv["YottaDB"):"S zr=$REFERENCE",1:"S zr=$ZREFERENCE")
 xecute sv
 new $etrap,$estack set $etrap="D fail^VSLRTAP" ; (1) fail-open trap BEFORE the guard read (B2)
 do work() ; (2) all risky work in a sub-call; any fault resumes at step (3)
 if zr'="" set zr=$data(@zr) ; (3) restore naked by re-reference (portable) — also the fault-resume target
 quit
 ;
work() ; risky body: guard, duration cap, then names-only capture
 new mode,exp,rpc,seq
 set mode=$get(^XTMP("VSLRT","ON")) if 'mode quit ; disarmed -> nothing
 set exp=$get(^XTMP("VSLRT","EXP")) ; in-path duration cap (C3a), reaper-independent
 if exp]"",$$past(exp) kill ^XTMP("VSLRT","ON") quit
 set rpc=$get(XWB(2,"RPC")) ; RPC name (XWBPRS PRS2; may be "" for a pre-name message)
 set seq=$increment(^XTMP("VSLRT","buf",$job,"seq")) ; per-job counter — NO shared node
 if seq=1 set ^XTMP("VSLRT","buf",$job,"inc")=$horolog_"-"_$get(DUZ) ; per-incarnation token (D13), stamped once
 set ^XTMP("VSLRT","buf",$job,seq)=$$rec(rpc)
 ; MODE=2 full-payload capParams + ring drop-oldest trim land in P1a.2
 quit
 ;
rsp() ; splice 17.5 — response capture. Same fence shape as req.
 new sv,zr set zr=""
 set sv=$select(($zv["GT.M")!($zv["YottaDB"):"S zr=$REFERENCE",1:"S zr=$ZREFERENCE")
 xecute sv
 new $etrap,$estack set $etrap="D fail^VSLRTAP"
 do workR()
 if zr'="" set zr=$data(@zr)
 quit
 ;
workR() ; guard, then (MODE=2) result capture. Names-only: nothing to add.
 new mode
 set mode=$get(^XTMP("VSLRT","ON")) if 'mode quit
 ; MODE=2 capResult (by effective XWBPTYPE) lands in P1a.2
 quit
 ;
fail() ; first error -> self-disable system-wide; clear $ECODE; emit ZERO I/O.
 ; Clearing $ECODE RESUMES in the caller (req/rsp) at the command after D work()
 ; -> the naked-ref restore + QUIT (clean exit); the broker proceeds byte-identically.
 kill ^XTMP("VSLRT","ON")
 set $ecode=""
 quit
 ;
rec(rpc) ; names-only raw record: ver ^ kind ^ $H ^ rpc-name
 quit "1^req^"_$horolog_"^"_rpc
 ;
past(exp) ; 1 if now ($H) is past the expiry <exp> (a $H value), else 0
 new d,nd,ns,s
 set nd=$piece($horolog,",",1),ns=$piece($horolog,",",2)
 set d=$piece(exp,",",1),s=$piece(exp,",",2)
 quit (nd>d)!((nd=d)&(ns>s))
