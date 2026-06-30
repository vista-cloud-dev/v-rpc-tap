VSLRTCTST ; VSL RPC TAP — P2 L-block ring CONCURRENCY + DURABILITY proofs (L4-L6)
 ; Run: m test tests/VSLRTCTST.m --engine ydb  --docker m-test-engine --routines src --routines ../m-stdlib/src
 ;      m test tests/VSLRTCTST.m --engine iris --docker m-test-iris   --routines src --routines ../m-stdlib/src
 ;
 ; Companion to VSLRTLTST (L1-L3 in-path safety). This suite proves the ring's
 ; multi-actor INVARIANTS — single trim owner (D8), per-incarnation segmentation
 ; (D13), durability watermark / at-least-once (D12/F-E) — that hold when the
 ; in-path writer, the off-path drainer, committrim, and the reaper all touch one
 ; ^XTMP("VSLRT","buf",$J,*) ring. No new production code: the fences pre-exist
 ; (VSLRTAP trim, VSLRTH drain/committrim, VSLRTRP orphans); this harness proves
 ; they compose without lost or doubled records.
 ;
 ; SCOPE / honesty (mirrors VSLRTLTST's class-agnostic note): the m-test harness
 ; is a SINGLE M job, so these proofs use DETERMINISTIC INTERLEAVING — each actor
 ; is driven by hand in every order that the real race window can produce. This is
 ; sound because the design has NO shared written node across writers (each $J owns
 ; its ring; $INCREMENT is atomic), so the only multi-actor surface is a single
 ; ring's head/wm/seq, whose orderings are finite and enumerated here. True
 ; 5000-process concurrency + throughput (fill-vs-drain knee) is the L7 load proof,
 ; not this suite.
 ;
 ; L4 single trim owner (D8/R11b): in-path trim drops-oldest past the depth cap but
 ;    NEVER crosses the drained watermark; committrim owns deletion of drained
 ;    records; a record is removed by EXACTLY ONE owner, head is monotonic, and a
 ;    reaper KILL of one (dead-$J) ring leaves a sibling live ring intact.
 ; L5 intra-arm $J-reuse segmentation (D13/F-F): across a reaper reap, a recycled
 ;    PID gets a FRESH (inc,$J) bucket so the two incarnations never conflate; and
 ;    WITHOUT a reap the ring keeps the old inc — proving the reaper is the
 ;    load-bearing segmentation mechanism (documented dependency, not a defect).
 ; L6 durability watermark (D12/F-E): drain sets the watermark and DELETES NOTHING,
 ;    so a host crash between drain and the committrim "ack" loses no record — the
 ;    same records re-drain (at-least-once) and carry the (inc,$J,seq) de-dup key;
 ;    only committrim (post-ack) deletes.
 new pass,fail
 do start^STDASSERT(.pass,.fail)
 ; L4 — single trim owner
 do tTrimNeverCrossesWatermark(.pass,.fail)
 do tTrimDropsCommittedAfterCommitTrim(.pass,.fail)
 do tTrimAndCommitTrimDisjointNoDouble(.pass,.fail)
 do tReaperKillOneRingSiblingIntact(.pass,.fail)
 ; L5 — intra-arm $J-reuse segmentation
 do tReapThenReuseFreshIncarnation(.pass,.fail)
 do tReuseWithoutReapKeepsOldInc(.pass,.fail)
 ; L6 — durability watermark / at-least-once
 do tDrainDeletesNothingRecordsSurvive(.pass,.fail)
 do tReDrainAfterCrashNoLoss(.pass,.fail)
 do tCommitTrimOnlyDeletesAfterAck(.pass,.fail)
 do tDedupKeyPresentAndStable(.pass,.fail)
 do report^STDASSERT(pass,fail)
 quit
 ;
seed(job,n) ; fabricate a ring for <job>: n req records, head=1, inc stamped
 new k
 kill ^XTMP("VSLRT","buf",job)
 set ^XTMP("VSLRT","buf",job,"seq")=n,^XTMP("VSLRT","buf",job,"head")=1
 set ^XTMP("VSLRT","buf",job,"inc")="1,1-1"
 for k=1:1:n set ^XTMP("VSLRT","buf",job,k)="1^req^1,1^RPC"_k_"^1"
 quit
 ;
present(job,lo,hi) ; 1 iff every ring node [lo..hi] for <job> exists, else 0
 new k,ok
 set ok=1
 for k=lo:1:hi if '$data(^XTMP("VSLRT","buf",job,k)) set ok=0 quit
 quit ok
 ;
absent(job,lo,hi) ; 1 iff NO ring node in [lo..hi] for <job> exists, else 0
 new k,ok
 set ok=1
 for k=lo:1:hi if $data(^XTMP("VSLRT","buf",job,k)) set ok=0 quit
 quit ok
 ;
 ; ---- L4: single trim owner (D8) ----
 ;
tTrimNeverCrossesWatermark(pass,fail) ;@TEST "L4/D8: in-path trim refuses to drop drained-uncommitted records (head never crosses wm)"
 new j set j=$job
 do seed(j,10)
 set ^XTMP("VSLRT","DEPTH")=5,^XTMP("VSLRT","buf",j,"wm")=5 ; cap 5, but 1..5 drained-not-committed
 do trim^VSLRTAP(j)
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"head")),1,"head pinned at 1 (cannot cross wm=5)")
 do true^STDASSERT(.pass,.fail,$$present(j,1,10),"all 10 records survive (no drained record lost)")
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","buf",j,"drop")),0,"zero drops while blocked by the watermark")
 kill ^XTMP("VSLRT")
 quit
 ;
tTrimDropsCommittedAfterCommitTrim(pass,fail) ;@TEST "L4/D8: once committrim acks [1..5], trim may drop past it (head advances, drop counted)"
 new j set j=$job
 do seed(j,10)
 set ^XTMP("VSLRT","DEPTH")=5,^XTMP("VSLRT","buf",j,"wm")=5
 do committrim^VSLRTH(j,5) ; host ack of 1..5 -> deleted, head=6
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"head")),6,"committrim advanced head to 6")
 set ^XTMP("VSLRT","buf",j,"seq")=11,^XTMP("VSLRT","buf",j,11)="1^req^1,1^RPC11^1" ; a fresh in-path append arrives (seq 11)
 do trim^VSLRTAP(j) ; depth now 6 (6..11) > cap 5 -> drop oldest live (6)
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"head")),7,"trim dropped seq 6, head=7")
 do true^STDASSERT(.pass,.fail,$$absent(j,1,6),"records 1..6 gone (5 committed, 6 trimmed)")
 do true^STDASSERT(.pass,.fail,$$present(j,7,11),"records 7..11 retained")
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","buf",j,"drop")),1,"exactly one in-path drop counted")
 kill ^XTMP("VSLRT")
 quit
 ;
tTrimAndCommitTrimDisjointNoDouble(pass,fail) ;@TEST "L4/D8: trim (pre-drain) and committrim (post-ack) remove disjoint ranges; head monotonic, no record killed twice"
 new j,h0,h1,h2 set j=$job
 do seed(j,20)
 set ^XTMP("VSLRT","DEPTH")=10 ; no wm yet (undrained) -> trim owns drop-oldest
 set h0=$get(^XTMP("VSLRT","buf",j,"head"))
 do trim^VSLRTAP(j) ; 20 live > cap 10 -> drop 1..10
 set h1=$get(^XTMP("VSLRT","buf",j,"head"))
 do eq^STDASSERT(.pass,.fail,h1,11,"trim advanced head 1 -> 11 (dropped 1..10)")
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","buf",j,"drop")),10,"10 pre-drain drops counted (visible loss, not silent)")
 do drain^VSLRTH(0,0) ; host drains 11..20 -> wm=20, deletes nothing
 do committrim^VSLRTH(j,20) ; ack 11..20 -> deleted
 set h2=$get(^XTMP("VSLRT","buf",j,"head"))
 do eq^STDASSERT(.pass,.fail,h2,21,"committrim advanced head 11 -> 21")
 do true^STDASSERT(.pass,.fail,(h0<h1)&(h1<h2),"head strictly monotonic across both owners")
 do true^STDASSERT(.pass,.fail,$$absent(j,1,20),"every record removed exactly once (none lingering, none doubled)")
 kill ^XTMP("VSLRT")
 quit
 ;
tReaperKillOneRingSiblingIntact(pass,fail) ;@TEST "L4: reaper KILL of a dead-$J ring leaves a live sibling ring's records intact (no cross-ring loss)"
 new dead,live set live=$job,dead=$$deadjob(live)
 do seed(dead,3),seed(live,4)
 do drain^VSLRTH(0,0) ; live ring drained (wm) — must not be disturbed by the reap
 do orphans^VSLRTRP() ; reaper sweep: dead-$J ring reaped, live $J ring kept
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",dead)),"dead-$J ring fully reaped")
 do true^STDASSERT(.pass,.fail,$$present(live,1,4),"live sibling ring's 4 records untouched")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",live,"wm")),4,"live ring watermark preserved across the reap")
 kill ^XTMP("VSLRT")
 quit
 ;
deadjob(live) ; a job number guaranteed NOT to be a live PID (live+offset, well outside the PID space)
 quit live+9000000
 ;
 ; ---- L5: intra-arm $J-reuse segmentation (D13/F-F) ----
 ;
tReapThenReuseFreshIncarnation(pass,fail) ;@TEST "L5/D13: across a reap, a recycled $J gets a fresh inc token + seq restarts -> incarnations never conflate"
 new j,inc1,inc2,XWB,DUZ set j=$job
 kill ^XTMP("VSLRT") set ^XTMP("VSLRT","ON")=2
 set XWB(2,"RPC")="X",DUZ=101
 do req^VSLRTAP() ; incarnation 1, seq 1, inc stamped from DUZ=101
 set inc1=$get(^XTMP("VSLRT","buf",j,"inc"))
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"seq")),1,"incarnation 1 at seq 1")
 kill ^XTMP("VSLRT","buf",j) ; reaper orphan-reap of the dead incarnation's ring
 set DUZ=202 ; the reused PID is a DIFFERENT user's signon
 do req^VSLRTAP() ; incarnation 2, seq restarts at 1, inc re-stamped from DUZ=202
 set inc2=$get(^XTMP("VSLRT","buf",j,"inc"))
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"seq")),1,"incarnation 2 restarts at seq 1")
 do true^STDASSERT(.pass,.fail,(inc1'="")&(inc2'="")&(inc1'=inc2),"distinct inc tokens segment the two incarnations")
 kill ^XTMP("VSLRT")
 quit
 ;
tReuseWithoutReapKeepsOldInc(pass,fail) ;@TEST "L5/F-F: reuse WITHOUT a reap keeps the old inc (conflation) -> the reaper is the load-bearing segmentation mechanism"
 new j,inc1,XWB,DUZ set j=$job
 kill ^XTMP("VSLRT") set ^XTMP("VSLRT","ON")=2
 set XWB(2,"RPC")="X",DUZ=101
 do req^VSLRTAP() ; incarnation 1, seq 1, inc1 stamped
 set inc1=$get(^XTMP("VSLRT","buf",j,"inc"))
 set DUZ=202 ; new user, same PID, ring NOT reaped
 do req^VSLRTAP() ; seq -> 2 (no restart), inc NOT re-stamped (only at seq=1)
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"seq")),2,"no reap -> seq continues to 2")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"inc")),inc1,"inc unchanged -> records conflated (reaper omission is the only failure mode)")
 kill ^XTMP("VSLRT")
 quit
 ;
 ; ---- L6: durability watermark / at-least-once (D12/F-E) ----
 ;
tDrainDeletesNothingRecordsSurvive(pass,fail) ;@TEST "L6/D12: drain sets the watermark and deletes NOTHING (records remain for re-drain)"
 new j set j=$job
 do seed(j,5)
 do drain^VSLRTH(0,0)
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"wm")),5,"drain set watermark to 5")
 do true^STDASSERT(.pass,.fail,$$present(j,1,5),"all 5 records still present after drain (no delete)")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"head")),1,"head not advanced by drain")
 kill ^XTMP("VSLRT")
 quit
 ;
tReDrainAfterCrashNoLoss(pass,fail) ;@TEST "L6/F-E: a crash between drain and committrim loses nothing; the same records re-drain (at-least-once), late appends included"
 new j set j=$job
 do seed(j,5)
 do drain^VSLRTH(0,0) ; first drain (wm=5)
 ; --- host crashes here: NO committrim ack --- in-path keeps appending 6..8
 set ^XTMP("VSLRT","buf",j,"seq")=8
 set ^XTMP("VSLRT","buf",j,6)="1^req^1,1^RPC6^1",^XTMP("VSLRT","buf",j,7)="1^req^1,1^RPC7^1",^XTMP("VSLRT","buf",j,8)="1^req^1,1^RPC8^1"
 ; host restarts and re-drains: nothing was deleted, so 1..5 are STILL here, plus 6..8
 do drain^VSLRTH(0,0)
 do true^STDASSERT(.pass,.fail,$$present(j,1,5),"original 1..5 re-drainable after the crash (no loss)")
 do true^STDASSERT(.pass,.fail,$$present(j,6,8),"appends 6..8 captured during the crash window")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"wm")),8,"watermark advanced to 8 on re-drain")
 kill ^XTMP("VSLRT")
 quit
 ;
tCommitTrimOnlyDeletesAfterAck(pass,fail) ;@TEST "L6/D12: only committrim (the post-PUT-ack step) deletes; after ack the ring drains empty"
 new j set j=$job
 do seed(j,5)
 do drain^VSLRTH(0,0)
 do committrim^VSLRTH(j,5) ; S3 PUT acked -> now delete 1..5
 do true^STDASSERT(.pass,.fail,$$absent(j,1,5),"committrim deleted 1..5 after ack")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"head")),6,"head advanced to 6")
 do drain^VSLRTH(0,0) ; re-drain post-ack yields nothing
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"wm")),5,"watermark stays at 5 (nothing new to drain)")
 kill ^XTMP("VSLRT")
 quit
 ;
tDedupKeyPresentAndStable(pass,fail) ;@TEST "L6/D12: every retained record exposes the (inc,$J,seq) de-dup key, stable across re-drains"
 new j,inc0 set j=$job
 do seed(j,3)
 set inc0=$get(^XTMP("VSLRT","buf",j,"inc"))
 do drain^VSLRTH(0,0),drain^VSLRTH(0,0) ; two drains (the at-least-once double-delivery case)
 do true^STDASSERT(.pass,.fail,inc0'="","inc component present (the $J-header de-dup field)")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"inc")),inc0,"inc stable across re-drains (drain never mutates it)")
 do true^STDASSERT(.pass,.fail,$$present(j,1,3),"seq components 1..3 intact -> (inc,$J,seq) reconstructable for de-dup")
 kill ^XTMP("VSLRT")
 quit
