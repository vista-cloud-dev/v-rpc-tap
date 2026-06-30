VSLRTAP ; VSL RPC TAP — in-path RPC capture (the splice body).
 ; doc: @exrun bare
 ; doc: @exsafe observe-only
 ; m-lint: disable-file=M-MOD-024 ; reads broker process vars by contract (XWB/XWBP/XWBPTYPE), as VSLFS/VSLIO read FDA by-ref
 ; m-lint: disable-file=M-MOD-036 ; gwalk @ is READ-ONLY global traversal of broker-named refs (never XECUTE); malformed refs fail safe under the trap
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
 set ^XTMP("VSLRT","buf",$job,seq)=$$rec(rpc,mode)
 if mode=2 do capParams(seq) ; full-payload params (XWB(5,"P",*) + referents, CF1/R16)
 do trim($job) ; head drop-oldest, watermark-respecting (D8)
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
 new mode,seq
 set mode=$get(^XTMP("VSLRT","ON")) if 'mode quit
 if mode'=2 quit ; names-only: rsp is a no-op
 set seq=$get(^XTMP("VSLRT","buf",$job,"seq")) if seq do capResult(seq) ; current RPC's seq (req just set it)
 quit
 ;
fail() ; first error -> self-disable system-wide; clear $ECODE; emit ZERO I/O.
 ; Clearing $ECODE RESUMES in the caller (req/rsp) at the command after D work()
 ; -> the naked-ref restore + QUIT (clean exit); the broker proceeds byte-identically.
 kill ^XTMP("VSLRT","ON")
 set $ecode=""
 quit
 ;
rec(rpc,mode) ; raw record: ver ^ kind ^ $H ^ rpc-name ^ mode
 quit "1^req^"_$horolog_"^"_rpc_"^"_mode
 ;
past(exp) ; 1 if now ($H) is past the expiry <exp> (a $H value), else 0
 new d,nd,ns,s
 set nd=$piece($horolog,",",1),ns=$piece($horolog,",",2)
 set d=$piece(exp,",",1),s=$piece(exp,",",2)
 quit (nd>d)!((nd=d)&(ns>s))
 ;
 ; ---- MODE=2 full-payload capture (CF1 / XWBRW SNDDATA) ----
 ; Storage under ^XTMP("VSLRT","buf",$J,seq): "P",ix[,"L"/"G",*] params; "R"[,n] result.
 ; Each node is SET individually (never one giant MERGE) -> MAXSTRING-safe (CF5/R1b).
 ;
capParams(seq) ; capture every input param XWB(5,"P",*) plus its referent
 new ix
 set ix=""
 for  set ix=$order(XWB(5,"P",ix)) quit:ix=""  do capParam(seq,ix)
 quit
 ;
capParam(seq,ix) ; one param: the descriptor/literal + (list-local | global) referent
 new ref,sub,v
 set v=XWB(5,"P",ix)
 set ^XTMP("VSLRT","buf",$job,seq,"P",ix)=v
 if $extract(v,1,5)=".XWBS" do  quit ; list param -> walk the local array (CF1 OARY/LINST)
 . set ref=$extract(v,2,$length(v)),sub=""
 . for  set sub=$order(@ref@(sub)) quit:sub=""  set ^XTMP("VSLRT","buf",$job,seq,"P",ix,"L",sub)=@ref@(sub)
 if $extract(v)="^" do gwalk(v,$name(^XTMP("VSLRT","buf",$job,seq,"P",ix,"G"))) ; global param (CF1 GINST)
 quit
 ;
capResult(seq) ; capture the result XWBP by EFFECTIVE (clamped) XWBPTYPE (XWBRW 6 types)
 new i,n,root,ty
 set ty=$select('$data(XWBPTYPE):1,XWBPTYPE<1:1,XWBPTYPE>6:1,1:XWBPTYPE) ; clamp (XWBTCPM:192)
 set ^XTMP("VSLRT","buf",$job,seq,"R")=ty
 if ty=1 set ^XTMP("VSLRT","buf",$job,seq,"R",1)=$get(XWBP) quit ; single value
 if ty=5 do  quit ; global instance: the single value at @XWBP
 . set root=$get(XWBP) if $extract(root)'="^" quit
 . set ^XTMP("VSLRT","buf",$job,seq,"R",1)=$get(@root)
 if ty=4 do gwalk($get(XWBP),$name(^XTMP("VSLRT","buf",$job,seq,"R"))) quit ; global array: traverse @XWBP
 set n=0,i="" ; types 2/3/6: subtree XWBP(I) in wire order
 for  set i=$order(XWBP(i)) quit:i=""  set n=n+1,^XTMP("VSLRT","buf",$job,seq,"R",n)=XWBP(i)
 quit
 ;
gwalk(root,dest) ; copy global subtree @root into dest(n) (n = $query sequence); dest is a global name
 new i,n,t
 if $extract(root)'="^" quit
 set t=$extract(root,1,$length(root)-1),n=0
 if $data(@root)>10 set n=n+1,@dest@(n)=@root ; non-null root node first (XWBRW:58)
 set i=root
 for  set i=$query(@i) quit:(i="")!(i'[t)  set n=n+1,@dest@(n)=@i
 quit
 ;
trim(job) ; head drop-oldest past the depth cap; never cross the drained watermark (D8/D12)
 new cap,head,seq,wm
 set cap=$get(^XTMP("VSLRT","DEPTH")) if 'cap set cap=10000
 set seq=$get(^XTMP("VSLRT","buf",job,"seq"))
 set head=$get(^XTMP("VSLRT","buf",job,"head")) if 'head set head=1
 set wm=+$get(^XTMP("VSLRT","buf",job,"wm")) ; drained watermark (drainer-owned; 0 = none)
 for  quit:(seq-head+1)'>cap  quit:head'>wm  do
 . kill ^XTMP("VSLRT","buf",job,head)
 . set head=head+1,^XTMP("VSLRT","buf",job,"drop")=1+$get(^XTMP("VSLRT","buf",job,"drop"))
 set ^XTMP("VSLRT","buf",job,"head")=head
 quit
