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
