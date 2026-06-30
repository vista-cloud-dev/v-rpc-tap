VSLRTLDTST ; VSL RPC TAP — P2 method-C ring/load stress: fill-vs-drain conservation + drop accounting (L9/L10)
 ; Run: m test tests/VSLRTLDTST.m --engine ydb  --docker m-test-engine --routines src --routines ../m-stdlib/src
 ;      m test tests/VSLRTLDTST.m --engine iris --docker m-test-iris   --routines src --routines ../m-stdlib/src
 ;
 ; METHOD C (deep analysis §6.3): drive the in-engine capture path (work^VSLRTAP) +
 ; the host seam (drain/committrim^VSLRTH) + the reaper (overflow/depth^VSLRTRP) at
 ; volume to stress the ring WITHOUT real sockets, the splice, or any install — so it
 ; needs no v-pkg. It does NOT test CF4 (no live socket); it is ring/journal-stress
 ; only. The wall-clock throughput knee (records/s) + ^XTMP journal-byte cost (L10)
 ; need engine-native instruments under the live 5000-load (L7) — out of scope here.
 ; What IS proven here, deterministically + dual-engine, is the CORRECTNESS core of L9:
 ;
 ; L9a fill <= drain (host keeps up): zero drops, every record delivered, ring empties.
 ; L9b fill  > drain (sustained overflow, no drain): drop-oldest holds live AT the cap;
 ;     the drop counter is EXACT (= appended - cap) -> conservation, NO silent loss.
 ; L9c stuck host (drained-but-uncommitted): the watermark forbids dropping undelivered
 ;     data, so NOTHING is lost even under overflow (the ring grows instead) -> the
 ;     reaper overflow() disarm is the backstop that bounds it.
 ; L10a multi-ring aggregation: depth/status sum exactly across many job rings; the
 ;     reaper's per-ring sweep visits them all.
 new pass,fail
 do start^STDASSERT(.pass,.fail)
 do tDrainKeepsUpZeroDrops(.pass,.fail)
 do tOverflowDropAccountingExact(.pass,.fail)
 do tStuckHostNoLossThenOverflowDisarm(.pass,.fail)
 do tMultiRingDepthAggregation(.pass,.fail)
 do report^STDASSERT(pass,fail)
 quit
 ;
arm(mode,cap) ; reset + arm names-only(1)/full(2) with depth cap <cap>; XWB name set for work()
 kill ^XTMP("VSLRT")
 set ^XTMP("VSLRT","ON")=mode,^XTMP("VSLRT","DEPTH")=cap
 quit
 ;
live(job) ; live record count in <job>'s ring (seq - head + 1)
 quit $get(^XTMP("VSLRT","buf",job,"seq"))-$get(^XTMP("VSLRT","buf",job,"head"),1)+1
 ;
 ; ---- L9a: host keeps up -> zero drops, exact delivery ----
 ;
tDrainKeepsUpZeroDrops(pass,fail) ;@TEST "L9a: drain+committrim keeping pace with fill -> 0 drops, all delivered, ring empties"
 new XWB,committed,i,j set j=$job
 do arm(1,1000) set XWB(2,"RPC")="LOADA",committed=0
 for i=1:1:5000 do work^VSLRTAP()  do:'(i#500) drainCommit(j,.committed) ; host acks every 500
 do drainCommit(j,.committed) ; final flush
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","buf",j,"drop")),0,"zero drops while the host keeps up")
 do eq^STDASSERT(.pass,.fail,committed,5000,"all 5000 records delivered + acked")
 do eq^STDASSERT(.pass,.fail,$$live(j),0,"ring fully drained (empty)")
 kill ^XTMP("VSLRT")
 quit
 ;
drainCommit(job,committed) ; host: drain the live range then committrim it; tally records acked
 new head,sm
 set head=$get(^XTMP("VSLRT","buf",job,"head"),1),sm=$get(^XTMP("VSLRT","buf",job,"seq"))
 quit:sm<head
 do drain^VSLRTH(0,0)
 set committed=committed+(sm-head+1)
 do committrim^VSLRTH(job,sm)
 quit
 ;
 ; ---- L9b: sustained overflow, no drain -> exact drop accounting, no silent loss ----
 ;
tOverflowDropAccountingExact(pass,fail) ;@TEST "L9b: fill >> drain (no drain) -> drop-oldest pins live AT cap; drop counter EXACT (= N-cap), no silent loss"
 new XWB,i,j set j=$job
 do arm(1,1000) set XWB(2,"RPC")="LOADB"
 for i=1:1:5000 do work^VSLRTAP() ; never drained
 do eq^STDASSERT(.pass,.fail,$$live(j),1000,"live pinned at the depth cap (1000)")
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","buf",j,"drop")),4000,"drop counter exact = appended - cap (5000-1000)")
 do eq^STDASSERT(.pass,.fail,$$live(j)+$get(^XTMP("VSLRT","buf",j,"drop")),5000,"conservation: live + dropped = appended (no silent loss)")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",j,"head")),4001,"head advanced exactly past the dropped prefix")
 kill ^XTMP("VSLRT")
 quit
 ;
 ; ---- L9c: stuck host (drained, never committed) -> watermark forbids loss; reaper bounds it ----
 ;
tStuckHostNoLossThenOverflowDisarm(pass,fail) ;@TEST "L9c: drained-but-uncommitted prefix is NEVER dropped under overflow (grows); reaper overflow() disarms as the backstop"
 new XWB,i,j set j=$job
 do arm(1,1000) set XWB(2,"RPC")="LOADC"
 for i=1:1:500 do work^VSLRTAP() ; fill 500
 do drain^VSLRTH(0,0) ; host drains (wm=500) but then STALLS — never committrims
 for i=1:1:4500 do work^VSLRTAP() ; keep filling to 5000, no acks
 do eq^STDASSERT(.pass,.fail,+$get(^XTMP("VSLRT","buf",j,"drop")),0,"ZERO drops: the watermark forbids discarding undelivered data")
 do true^STDASSERT(.pass,.fail,$$live(j)>1000,"ring grew past the cap (can't self-limit while blocked by wm)")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","buf",j,1)),"the oldest drained-uncommitted record still present (not lost)")
 ; the reaper overflow backstop bounds the unbounded growth by disarming
 set ^XTMP("VSLRT","OVCAP")=2000,^XTMP("VSLRT","health","lastdepth")=0
 do overflow^VSLRTRP()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"reaper overflow() disarmed capture (the stuck-host backstop)")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","health","overflow")),1,"overflow flagged for the doctor")
 kill ^XTMP("VSLRT")
 quit
 ;
 ; ---- L10a: multi-ring aggregation at scale ----
 ;
tMultiRingDepthAggregation(pass,fail) ;@TEST "L10a: depth/status aggregate exactly across many job rings; reaper sweep visits all"
 new exp,k,n,s set n=60,exp=0
 kill ^XTMP("VSLRT") set ^XTMP("VSLRT","ON")=1
 for k=1:1:n do  ; n synthetic rings of varying live depth k
 . set ^XTMP("VSLRT","buf",k,"seq")=k,^XTMP("VSLRT","buf",k,"head")=1,^XTMP("VSLRT","buf",k,"inc")="1,1-1"
 . set exp=exp+k
 do eq^STDASSERT(.pass,.fail,$$depth^VSLRTRP(),exp,"reaper depth() sums every ring (1+2+...+60)")
 set s=$$status^VSLRTH()
 do eq^STDASSERT(.pass,.fail,$piece(s,"^",3),"jobs="_n,"status counts all "_n_" rings")
 do eq^STDASSERT(.pass,.fail,$piece(s,"^",4),"records="_exp,"status sums all records")
 kill ^XTMP("VSLRT")
 quit
