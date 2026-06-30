VSLRTLTST ; VSL RPC TAP — P2 L-block in-path SAFETY proofs (L1-L3 + L2 fault injection)
 ; Run: m test tests/VSLRTLTST.m --engine ydb  --docker m-test-engine --routines src --routines ../m-stdlib/src
 ;      m test tests/VSLRTLTST.m --engine iris --docker m-test-iris   --routines src --routines ../m-stdlib/src
 ;
 ; Proves the fences ALREADY built into VSLRTAP (P1a) under deliberate fault, on
 ; BOTH engines through the driver stack. No new production code — this is the
 ; P2 safety harness (plan §4 / tracker L1-L3). The 5000-user load proof (L7) and
 ; the live-broker-nested half are later increments (BB2 topology); these are the
 ; highest-value correctness gates and need no rig.
 ;
 ; L1  trap absorbed, NOT propagated — a fault in capture is caught by VSLRTAP's own
 ;     $ETRAP and resumes in req()/rsp(); an OUTER (broker-emulating) $ETRAP never
 ;     fires and the broker-contract vars + control flow are byte-identical (CF4/B2).
 ; L2  forced-fault -> self-disable + ZERO bytes on the wire. The fail-open trap is
 ;     CLASS-AGNOSTIC (one $ETRAP wraps the whole risky body), so two deterministic
 ;     malformed-reference faults (bad global referent -> UNDEF/NAKED; bad list
 ;     referent -> SYNTAX/SUBSCRIPT), injected in BOTH the req and rsp paths, prove
 ;     the invariant for every class. Coverage of the named classes:
 ;       UNDEFINED/NAKED  -> tFaultBadGlobalRef* (deterministic, both engines)
 ;       SYNTAX/SUBSCRIPT -> tFaultBadListRef    (deterministic, both engines)
 ;       MAXSTRING        -> NOT a portable in-path trigger (YDB max 1MB vs IRIS
 ;                           ~3.6MB; the capture body only SETs values it reads, it
 ;                           never concatenates beyond rec()'s small pieces) -> the
 ;                           class-agnostic trap covers it; a natural giant-result
 ;                           trigger lands with the live-broker phase.
 ;       I/O              -> STRUCTURALLY ABSENT: the in-path body (req/work/rsp/workR/
 ;                           capParams/capResult/gwalk/trim/rec) contains no
 ;                           WRITE/READ/OPEN/USE/CLOSE — the very invariant L2 asserts
 ;                           (zero $X/$Y movement) is the proof.
 ; L3  naked-reference integrity preserved across req/rsp — disarmed, armed-clean,
 ;     and armed-FAULT (the fault-resume path still restores the naked ref).
 new pass,fail
 do start^STDASSERT(.pass,.fail)
 ; L1 — trap absorbed, not propagated; non-interference
 do tTrapAbsorbedNotPropagated(.pass,.fail)
 do tBrokerVarsUntouchedOnFault(.pass,.fail)
 ; L2 — forced fault -> self-disable + zero I/O (req + rsp paths)
 do tFaultBadGlobalRefReq(.pass,.fail)
 do tFaultBadListRefReq(.pass,.fail)
 do tFaultBadGlobalRefRsp(.pass,.fail)
 do tFaultZeroIoReq(.pass,.fail)
 do tFaultZeroIoRsp(.pass,.fail)
 ; L3 — naked-ref integrity under fault
 do tNakedRefArmedFault(.pass,.fail)
 do report^STDASSERT(pass,fail)
 quit
 ;
arm(mode) ; reset tap state, arm at <mode>
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","ON")=mode
 quit
 ;
 ; ---- L1: trap absorbed locally, broker proceeds byte-identically ----
 ;
tTrapAbsorbedNotPropagated(pass,fail) ;@TEST "L1: capture fault is absorbed by VSLRTAP trap; outer broker $ETRAP never fires, control returns"
 new XWB,outer,sentinel
 do arm(2)
 ; a bad global referent param -> faults inside capParams (designed fail-safe path)
 set XWB(2,"RPC")="ORWU DT",XWB(5,"P",1)="^"
 set outer=0,sentinel=0
 new $etrap,$estack set $etrap="set outer=1"
 do req^VSLRTAP()
 set sentinel=1 ; reached only if req returned normally (no unwind to the outer trap)
 do eq^STDASSERT(.pass,.fail,outer,0,"outer broker trap NEVER fired (fault absorbed locally)")
 do eq^STDASSERT(.pass,.fail,sentinel,1,"control returned to caller normally (broker proceeds)")
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"tap self-disabled on first fault")
 do eq^STDASSERT(.pass,.fail,$ecode,"","$ECODE clean after absorbed fault")
 quit
 ;
tBrokerVarsUntouchedOnFault(pass,fail) ;@TEST "L1: broker-contract vars (XWB/XWBP/XWBPTYPE) are byte-identical after an absorbed fault"
 new XWB,XWBP,XWBPTYPE
 do arm(2)
 set XWB(2,"RPC")="ORWU DT",XWB(5,"P",1)="^",XWBP="RESULT",XWBPTYPE=1
 do req^VSLRTAP()
 do eq^STDASSERT(.pass,.fail,$get(XWB(2,"RPC")),"ORWU DT","XWB RPC name untouched")
 do eq^STDASSERT(.pass,.fail,$get(XWB(5,"P",1)),"^","XWB param untouched")
 do eq^STDASSERT(.pass,.fail,$get(XWBP),"RESULT","XWBP untouched")
 do eq^STDASSERT(.pass,.fail,$get(XWBPTYPE),1,"XWBPTYPE untouched")
 quit
 ;
 ; ---- L2: forced fault -> self-disable + ZERO bytes on the wire ----
 ;
tFaultBadGlobalRefReq(pass,fail) ;@TEST "L2: bad global referent in req -> trap fires, tap self-disables (UNDEF/NAKED class)"
 new XWB
 do arm(2)
 set XWB(2,"RPC")="X",XWB(5,"P",1)="^"
 do req^VSLRTAP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"self-disabled on bad-global-ref fault")
 do eq^STDASSERT(.pass,.fail,$ecode,"","$ECODE clean")
 quit
 ;
tFaultBadListRefReq(pass,fail) ;@TEST "L2: malformed list referent in req -> trap fires, tap self-disables (SYNTAX/SUBSCRIPT class)"
 new XWB
 do arm(2)
 set XWB(2,"RPC")="X",XWB(5,"P",1)=".XWBS1(" ; malformed .XWBSn descriptor -> @ref indirection errors
 do req^VSLRTAP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"self-disabled on bad-list-ref fault")
 do eq^STDASSERT(.pass,.fail,$ecode,"","$ECODE clean")
 quit
 ;
tFaultBadGlobalRefRsp(pass,fail) ;@TEST "L2: bad global referent in rsp (capResult) -> trap fires, tap self-disables"
 new XWB,XWBP,XWBPTYPE
 do arm(2)
 set XWB(2,"RPC")="X" do req^VSLRTAP() ; clean req establishes seq
 set XWBPTYPE=4,XWBP="^" ; type-4 global-array traverse of "^" -> faults in gwalk
 do rsp^VSLRTAP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"self-disabled on rsp fault")
 do eq^STDASSERT(.pass,.fail,$ecode,"","$ECODE clean")
 quit
 ;
tFaultZeroIoReq(pass,fail) ;@TEST "L2/CF4: req emits ZERO device bytes under fault ($X/$Y do not move)"
 ; capture $X/$Y INTO LOCALS straddling only the req call — STDASSERT's own
 ; PASS-line output moves $X/$Y, so reading them live in the 2nd assert would
 ; measure the harness, not the tap. The locals isolate the req-call delta.
 new XWB,x0,x1,y0,y1
 do arm(2)
 set XWB(2,"RPC")="X",XWB(5,"P",1)="^"
 set x0=$x,y0=$y
 do req^VSLRTAP()
 set x1=$x,y1=$y
 do eq^STDASSERT(.pass,.fail,x1,x0,"$X unchanged across faulting req (no wire write)")
 do eq^STDASSERT(.pass,.fail,y1,y0,"$Y unchanged across faulting req")
 quit
 ;
tFaultZeroIoRsp(pass,fail) ;@TEST "L2/CF4: rsp emits ZERO device bytes under fault ($X/$Y do not move)"
 new XWB,XWBP,XWBPTYPE,x0,x1,y0,y1
 do arm(2)
 set XWB(2,"RPC")="X" do req^VSLRTAP()
 set XWBPTYPE=4,XWBP="^"
 set x0=$x,y0=$y
 do rsp^VSLRTAP()
 set x1=$x,y1=$y ; capture before any assert prints (see tFaultZeroIoReq note)
 do eq^STDASSERT(.pass,.fail,x1,x0,"$X unchanged across faulting rsp (no wire write)")
 do eq^STDASSERT(.pass,.fail,y1,y0,"$Y unchanged across faulting rsp")
 quit
 ;
 ; ---- L3: naked-reference integrity under fault ----
 ;
tNakedRefArmedFault(pass,fail) ;@TEST "L3: naked ref restored across req even when capture faults mid-flight (R2 fence on the fault-resume path)"
 new XWB,got
 do arm(2)
 set XWB(2,"RPC")="X",XWB(5,"P",1)="^" ; will fault inside capParams
 kill ^ZZRT set ^ZZRT("a",1)=11,^ZZRT("b",1)=22
 set got=^ZZRT("b",1) ; naked indicator now at ^ZZRT("b",_)
 do req^VSLRTAP()
 set got=^(1) ; re-reference: fence held iff ^(1)=^ZZRT("b",1)=22
 do eq^STDASSERT(.pass,.fail,got,22,"naked ref restored after a FAULTING armed req")
 kill ^ZZRT
 quit
