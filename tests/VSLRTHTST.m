VSLRTHTST ; VSL RPC TAP — VSLRTH off-path host seam tests (YDB+IRIS)
 ; Run: m test tests/VSLRTHTST.m --engine ydb  --docker m-test-engine --routines src --routines ../m-stdlib/src
 ;      m test tests/VSLRTHTST.m --engine iris --docker m-test-iris   --routines src --routines ../m-stdlib/src
 new pass,fail
 do start^STDASSERT(.pass,.fail)
 do tArmSetsState(.pass,.fail)
 do tArmDuration(.pass,.fail)
 do tDisarm(.pass,.fail)
 do tStatus(.pass,.fail)
 do tCommitTrim(.pass,.fail)
 do tCommitTrimClamp(.pass,.fail)
 do tDrainSetsWatermarkNoDelete(.pass,.fail)
 do tDrainRangeScopesJobs(.pass,.fail)
 do report^STDASSERT(pass,fail)
 quit
 ;
seed(job,n) ; fabricate a ring for <job>: n req records, head=1
 new k
 kill ^XTMP("VSLRT","buf",job)
 set ^XTMP("VSLRT","buf",job,"seq")=n,^XTMP("VSLRT","buf",job,"head")=1
 set ^XTMP("VSLRT","buf",job,"inc")="1,1-"
 for k=1:1:n set ^XTMP("VSLRT","buf",job,k)="1^req^1,1^RPC"_k_"^1"
 quit
 ;
tArmSetsState(pass,fail) ;@TEST "arm sets mode flag + lease + epoch + purge node (no EXP when dur=0)"
 kill ^XTMP("VSLRT")
 do arm^VSLRTH(1,90,0)
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","ON")),1,"mode flag = 1")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","LEASE")),"lease set")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","EPOCH")),"epoch set")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT",0)),"purge-date node set (A9)")
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","EXP")),"no EXP when duration 0")
 quit
 ;
tArmDuration(pass,fail) ;@TEST "arm with duration sets the absolute expiry (C3a)"
 kill ^XTMP("VSLRT")
 do arm^VSLRTH(2,90,3600)
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","ON")),2,"mode flag = 2")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","EXP")),"EXP set when duration > 0")
 quit
 ;
tDisarm(pass,fail) ;@TEST "disarm clears the mode flag (reaper self-terminates)"
 kill ^XTMP("VSLRT")
 do arm^VSLRTH(1,90,0)
 do disarm^VSLRTH()
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","ON")),"mode flag cleared")
 quit
 ;
tStatus(pass,fail) ;@TEST "status reports armed state + job/record counts"
 kill ^XTMP("VSLRT")
 do arm^VSLRTH(1,90,0)
 do seed(100,2),seed(200,3)
 new s set s=$$status^VSLRTH()
 do eq^STDASSERT(.pass,.fail,$piece(s,"^",1),"on=1","status: on=1")
 do eq^STDASSERT(.pass,.fail,$piece(s,"^",3),"jobs=2","status: jobs=2")
 do eq^STDASSERT(.pass,.fail,$piece(s,"^",4),"records=5","status: records=5")
 quit
 ;
tCommitTrim(pass,fail) ;@TEST "committrim deletes [head..seq] and advances head (D12/F-E)"
 kill ^XTMP("VSLRT") do seed(100,5)
 do committrim^VSLRTH(100,3)
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",100,1)),"seq 1 deleted")
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",100,3)),"seq 3 deleted")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","buf",100,4)),"seq 4 retained")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",100,"head")),4,"head advanced to 4")
 quit
 ;
tCommitTrimClamp(pass,fail) ;@TEST "committrim clamps seq to the live max (no over-advance)"
 kill ^XTMP("VSLRT") do seed(100,3)
 do committrim^VSLRTH(100,9)
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",100,3)),"seq 3 deleted")
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",100,"head")),4,"head clamped to seqmax+1")
 quit
 ;
tDrainSetsWatermarkNoDelete(pass,fail) ;@TEST "drain sets the watermark and does NOT delete (D12)"
 kill ^XTMP("VSLRT") do seed(100,3)
 do drain^VSLRTH(0,0)
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",100,"wm")),3,"watermark = seqmax")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","buf",100,1)),"seq 1 NOT deleted (read-only)")
 do true^STDASSERT(.pass,.fail,$data(^XTMP("VSLRT","buf",100,3)),"seq 3 NOT deleted")
 quit
 ;
tDrainRangeScopesJobs(pass,fail) ;@TEST "drain(lo,hi) only touches jobs in range"
 kill ^XTMP("VSLRT") do seed(100,2),seed(200,2)
 do drain^VSLRTH(100,150)
 do eq^STDASSERT(.pass,.fail,$get(^XTMP("VSLRT","buf",100,"wm")),2,"job 100 drained")
 do true^STDASSERT(.pass,.fail,'$data(^XTMP("VSLRT","buf",200,"wm")),"job 200 out of range, untouched")
 quit
