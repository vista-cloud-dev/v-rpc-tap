VSLRTAPTST ; VSL RPC TAP — VSLRTAP in-path capture tests (YDB+IRIS; P1a names-only)
 ; Run: m test tests/VSLRTAPTST.m --engine ydb  --docker m-test-engine --routines src --routines ../m-stdlib/src
 ;      m test tests/VSLRTAPTST.m --engine iris --docker m-test-iris   --routines src --routines ../m-stdlib/src
 ; Record schema v1 at ^XTMP("VSLRT","buf",$J,seq) = ver^kind^$H^rpc
 ;   ($J + seq live in the subscript; the per-incarnation token is the sibling "inc" node)
 new pass,fail
 do start^STDASSERT(.pass,.fail)
 do tDisarmedNoop(.pass,.fail)
 do tNamesOnlyCapture(.pass,.fail)
 do tSeqIncrements(.pass,.fail)
 do tRspNamesOnlyNoop(.pass,.fail)
 do tNakedRefArmed(.pass,.fail)
 do tNakedRefDisarmed(.pass,.fail)
 do tDurationCapExpires(.pass,.fail)
 do tFailSelfDisables(.pass,.fail)
 do tFullParamsLiteral(.pass,.fail)
 do tFullParamsListReferent(.pass,.fail)
 do tFullParamsGlobalReferent(.pass,.fail)
 do tFullResultSingle(.pass,.fail)
 do tFullResultArray(.pass,.fail)
 do tFullResultGlobalArray(.pass,.fail)
 do tFullResultGlobalInstance(.pass,.fail)
 do tFullResultTypeClamp(.pass,.fail)
 do tTrimDropsOldest(.pass,.fail)
 do tTrimRespectsWatermark(.pass,.fail)
 do report^STDASSERT(pass,fail)
 quit
 ;
arm(mode) ; reset tap state, arm at <mode> (1=names-only, 2=full)
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","ON")=mode
 quit
 ;
tDisarmedNoop(pass,fail) ;@TEST "disarmed: req writes nothing, leaves no ring"
 kill ^XTMP("VSLRT")
 new XWB set XWB(2,"RPC")="ORWU DT"
 do req^VSLRTAP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",$job)),"no ring node when OFF")
 quit
 ;
tNamesOnlyCapture(pass,fail) ;@TEST "names-only: req captures rpc name, seq=1, inc token stamped"
 do arm(1)
 new XWB set XWB(2,"RPC")="ORWU DT"
 do req^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,"seq")),1,"seq=1")
 do eq^STDASSERT(.pass,.fail,$piece($get(^XTMP("VSLRT","buf",$job,1)),"^",2),"req","kind=req")
 do eq^STDASSERT(.pass,.fail,$piece($get(^XTMP("VSLRT","buf",$job,1)),"^",4),"ORWU DT","rpc name captured")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","buf",$job,"inc")),"inc token stamped")
 quit
 ;
tSeqIncrements(pass,fail) ;@TEST "second req increments seq; inc token stamped once"
 do arm(1)
 new XWB,inc1 set XWB(2,"RPC")="ORWU DT"
 do req^VSLRTAP()
 set inc1=$get(^XTMP("VSLRT","buf",$job,"inc"))
 set XWB(2,"RPC")="ORQQVI VITALS"
 do req^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,"seq")),2,"seq=2")
 do eq^STDASSERT(.pass,.fail,$piece($get(^XTMP("VSLRT","buf",$job,2)),"^",4),"ORQQVI VITALS","2nd rpc name")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,"inc")),inc1,"inc token unchanged (stamped once)")
 quit
 ;
tRspNamesOnlyNoop(pass,fail) ;@TEST "names-only: rsp adds nothing (MODE=1 no-op)"
 do arm(1)
 new XWB set XWB(2,"RPC")="ORWU DT"
 do req^VSLRTAP()
 do rsp^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,"seq")),1,"seq still 1 after rsp")
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",$job,1,"R")),"no result node names-only")
 quit
 ;
tNakedRefArmed(pass,fail) ;@TEST "naked ref preserved across req when armed (R2 fence)"
 do arm(1)
 new XWB,got set XWB(2,"RPC")="ORWU DT"
 kill ^ZZRT set ^ZZRT("a",1)=11,^ZZRT("b",1)=22
 set got=^ZZRT("b",1) ; naked indicator now at ^ZZRT("b",_)
 do req^VSLRTAP()
 set got=^(1) ; re-reference: if the fence held, ^(1)=^ZZRT("b",1)=22
 do eq^STDASSERT(.pass,.fail,got,22,"naked ref restored after armed req")
 kill ^ZZRT
 quit
 ;
tNakedRefDisarmed(pass,fail) ;@TEST "naked ref preserved across req when disarmed (B2 fence)"
 kill ^XTMP("VSLRT")
 new XWB,got set XWB(2,"RPC")="ORWU DT"
 kill ^ZZRT set ^ZZRT("a",1)=11,^ZZRT("b",1)=22
 set got=^ZZRT("b",1)
 do req^VSLRTAP()
 set got=^(1)
 do eq^STDASSERT(.pass,.fail,got,22,"naked ref restored after disarmed req")
 kill ^ZZRT
 quit
 ;
tDurationCapExpires(pass,fail) ;@TEST "in-path duration cap: past EXP self-disables, no capture (C3a)"
 do arm(1)
 set ^XTMP("VSLRT","EXP")="1,1" ; a $H far in the past
 new XWB set XWB(2,"RPC")="ORWU DT"
 do req^VSLRTAP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"tap self-disabled past expiry")
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",$job,1)),"no record written past expiry")
 quit
 ;
tFailSelfDisables(pass,fail) ;@TEST "fail-open: handler kills the mode flag and clears $ECODE"
 do arm(1)
 do fail^VSLRTAP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"mode flag cleared (self-disable)")
 do eq^STDASSERT(.pass,.fail,$ecode,"","$ECODE cleared")
 quit
 ;
 ; ---- P1a.2: MODE=2 full-payload (capParams/capResult) + ring trim ----
 ; MODE=2 storage contract (under ^XTMP("VSLRT","buf",$J,seq)):
 ;   "P",ix            = the param descriptor/literal (literal | ".XWBSn" | "^...ref")
 ;   "P",ix,"L",sub    = list referent values (local array, subscript preserved)
 ;   "P",ix,"G",n      = global referent values ($query order, n=sequence)
 ;   "R"               = effective (clamped) XWBPTYPE
 ;   "R",n             = result data lines in wire order (n=sequence)
 ;
tFullParamsLiteral(pass,fail) ;@TEST "MODE2: literal param captured verbatim"
 do arm(2)
 new XWB set XWB(2,"RPC")="ORWU DT",XWB(5,"P",1)="20260629"
 do req^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"P",1)),"20260629","literal param")
 quit
 ;
tFullParamsListReferent(pass,fail) ;@TEST "MODE2: list param walks the .XWBSn local referent"
 do arm(2)
 new XWB,XWBS1 set XWB(2,"RPC")="X",XWB(5,"P",1)=".XWBS1"
 set XWBS1(1)="alpha",XWBS1(2)="beta"
 do req^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"P",1)),".XWBS1","list descriptor")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"P",1,"L",1)),"alpha","list[1]")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"P",1,"L",2)),"beta","list[2]")
 quit
 ;
tFullParamsGlobalReferent(pass,fail) ;@TEST "MODE2: global param walks the ^TMP referent subtree"
 do arm(2)
 new XWB set XWB(2,"RPC")="X",XWB(5,"P",1)=$name(^TMP("XWBA",$job,1))
 kill ^TMP("XWBA",$job,1) set ^TMP("XWBA",$job,1,1)="g1",^TMP("XWBA",$job,1,2)="g2"
 do req^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"P",1,"G",1)),"g1","global[1]")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"P",1,"G",2)),"g2","global[2]")
 kill ^TMP("XWBA",$job,1)
 quit
 ;
tFullResultSingle(pass,fail) ;@TEST "MODE2 result type 1: single value"
 do arm(2)
 new XWB,XWBP,XWBPTYPE set XWB(2,"RPC")="X",XWBP="SINGLE",XWBPTYPE=1
 do req^VSLRTAP() do rsp^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R")),1,"type=1")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R",1)),"SINGLE","single value")
 quit
 ;
tFullResultArray(pass,fail) ;@TEST "MODE2 result type 2: array subtree in order"
 do arm(2)
 new XWB,XWBP,XWBPTYPE set XWB(2,"RPC")="X",XWBPTYPE=2,XWBP(1)="r1",XWBP(2)="r2"
 do req^VSLRTAP() do rsp^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R",1)),"r1","row1")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R",2)),"r2","row2")
 quit
 ;
tFullResultGlobalArray(pass,fail) ;@TEST "MODE2 result type 4: traverse @XWBP global array"
 do arm(2)
 new XWB,XWBP,XWBPTYPE set XWB(2,"RPC")="X",XWBPTYPE=4,XWBP="^ZZGA"
 kill ^ZZGA set ^ZZGA(1)="x",^ZZGA(2)="y"
 do req^VSLRTAP() do rsp^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R",1)),"x","ga node1")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R",2)),"y","ga node2")
 kill ^ZZGA
 quit
 ;
tFullResultGlobalInstance(pass,fail) ;@TEST "MODE2 result type 5: single value at @XWBP node"
 do arm(2)
 new XWB,XWBP,XWBPTYPE set XWB(2,"RPC")="X",XWBPTYPE=5,XWBP="^ZZGI(1)"
 kill ^ZZGI set ^ZZGI(1)="inst"
 do req^VSLRTAP() do rsp^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R",1)),"inst","instance value")
 kill ^ZZGI
 quit
 ;
tFullResultTypeClamp(pass,fail) ;@TEST "MODE2: out-of-range XWBPTYPE clamps to 1 (XWBTCPM:192)"
 do arm(2)
 new XWB,XWBP,XWBPTYPE set XWB(2,"RPC")="X",XWBP="clamped",XWBPTYPE=9
 do req^VSLRTAP() do rsp^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R")),1,"type 9 -> clamp 1")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,1,"R",1)),"clamped","captured as single")
 quit
 ;
tTrimDropsOldest(pass,fail) ;@TEST "ring trim: head drop-oldest past the depth cap (D8); drop counted"
 do arm(1)
 set ^XTMP("VSLRT","DEPTH")=3
 new XWB,i set XWB(2,"RPC")="R"
 for i=1:1:5 do req^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,"head")),3,"head advanced to 3")
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",$job,1)),"seq 1 dropped")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","buf",$job,5)),"seq 5 retained")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,"drop")),2,"2 drops counted")
 quit
 ;
tTrimRespectsWatermark(pass,fail) ;@TEST "ring trim: never deletes at/below the drained watermark (D8/D12)"
 do arm(1)
 set ^XTMP("VSLRT","DEPTH")=2
 new XWB,i set XWB(2,"RPC")="R"
 do req^VSLRTAP() do req^VSLRTAP()
 set ^XTMP("VSLRT","buf",$job,"wm")=5
 do req^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",$job,"head")),1,"head pinned at 1 by watermark")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","buf",$job,1)),"seq 1 retained (<=wm)")
 quit
