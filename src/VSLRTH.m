VSLRTH ; VSL RPC TAP — off-path host seam (arm/disarm/status/drain/committrim).
 ; doc: @exrun bare
 ; doc: @exsafe transactional
 ; m-lint: disable-file=M-MOD-036 ; dump @ indirection is READ-ONLY traversal of our own ^XTMP("VSLRT") ring (never XECUTE)
 ;
 ; The host (`v rpc-tap`, Go) reaches the engine ONLY through these labels over the
 ; mdriver/v-pkg seam. This runs OFF the live RPC path (in the driver job), so device
 ; I/O here is fine — the zero-I/O rule (CF4) binds VSLRTAP on the broker socket, not
 ; VSLRTH. Returns RAW bytes; ALL encoding/correlation/crypto/JSON is host-side (D10).
 ;
 ; drain wire format (one line per node; tab-delimited; \n-terminated):
 ;   J <TAB> job <TAB> inc <TAB> head <TAB> seqmax        — per-job header
 ;   V <TAB> job <TAB> seq <TAB> subpath <TAB> value      — one ring node ($C(2)-joined subpath)
 ; subpath = the subscripts BEYOND ...,buf,job,seq ("" = the record node itself).
 quit
 ;
arm(mode,ttl,dur) ; host: arm capture. mode=1|2; ttl=lease secs; dur=duration secs (0=indefinite)
 set mode=+$get(mode),ttl=+$get(ttl),dur=+$get(dur)
 set ^XTMP("VSLRT",0)=($piece($horolog,",",1)+7)_"^"_$horolog ; purge-date node (A9): purgedate^createdate
 set ^XTMP("VSLRT","EPOCH")=$horolog ; coarse arm marker (C2)
 set ^XTMP("VSLRT","LEASE")=$$plus($horolog,ttl) ; host-lease dead-man (reaper checks)
 if dur set ^XTMP("VSLRT","EXP")=$$plus($horolog,dur) ; absolute duration cap (C3a)
 set ^XTMP("VSLRT","ON")=mode ; ARM LAST — every guard exists before any capture can fire
 quit
 ;
disarm() ; host: stop capture (the reaper self-terminates on its next wake)
 kill ^XTMP("VSLRT","ON")
 quit
 ;
status() ; host doctor summary: "on=<m>^epoch=<$H>^jobs=<n>^records=<n>"
 new j,jobs,rec
 set jobs=0,rec=0,j=""
 for  set j=$order(^XTMP("VSLRT","buf",j)) quit:j=""  do
 . set jobs=jobs+1
 . set rec=rec+($get(^XTMP("VSLRT","buf",j,"seq"))-$get(^XTMP("VSLRT","buf",j,"head"),1)+1)
 quit "on="_$get(^XTMP("VSLRT","ON"))_"^epoch="_$get(^XTMP("VSLRT","EPOCH"))_"^jobs="_jobs_"^records="_rec
 ;
committrim(job,seq) ; host: delete the durably-stored prefix [head..seq], advance head (D12/F-E)
 new head,k,sm
 set head=$get(^XTMP("VSLRT","buf",job,"head"),1)
 set sm=$get(^XTMP("VSLRT","buf",job,"seq"))
 set:seq>sm seq=sm ; clamp to the live max — never advance head past what exists
 for k=head:1:seq kill ^XTMP("VSLRT","buf",job,k)
 set ^XTMP("VSLRT","buf",job,"head")=seq+1
 quit
 ;
drain(lo,hi) ; host: dump live records for jobs in [lo,hi] (0,0=all) to device; set per-job wm; NO delete (D12)
 new head,j,sm
 set lo=+$get(lo),hi=+$get(hi),j=""
 for  set j=$order(^XTMP("VSLRT","buf",j)) quit:j=""  do
 . quit:lo&(j<lo)
 . quit:hi&(j>hi)
 . set head=$get(^XTMP("VSLRT","buf",j,"head"),1),sm=$get(^XTMP("VSLRT","buf",j,"seq"))
 . write "J",$char(9),j,$char(9),$get(^XTMP("VSLRT","buf",j,"inc")),$char(9),head,$char(9),sm,!
 . do dumpJob(j,head,sm)
 . set ^XTMP("VSLRT","buf",j,"wm")=sm ; drained-up-to watermark; in-path trim won't cross it
 quit
 ;
dumpJob(job,head,sm) ; emit every live ring node for <job> (seq head..sm), skipping committrim'd gaps
 new ref,seq
 for seq=head:1:sm do
 . quit:'$data(^XTMP("VSLRT","buf",job,seq))
 . do dumpNode(job,seq,$name(^XTMP("VSLRT","buf",job,seq)))
 . set ref=$name(^XTMP("VSLRT","buf",job,seq))
 . for  set ref=$query(@ref) quit:ref=""  quit:$qsubscript(ref,4)'=seq  do dumpNode(job,seq,ref)
 quit
 ;
dumpNode(job,seq,ref) ; one node -> "V<TAB>job<TAB>seq<TAB>subpath<TAB>value"
 new d,p,sp
 set d=$qlength(ref),sp=""
 for p=5:1:d set sp=sp_$select(p>5:$char(2),1:"")_$qsubscript(ref,p)
 write "V",$char(9),job,$char(9),seq,$char(9),sp,$char(9),$get(@ref),!
 quit
 ;
plus(h,secs) ; return the $H value <h> advanced by <secs> seconds (handles day rollover)
 new d,s
 set d=$piece(h,",",1),s=$piece(h,",",2)+secs
 for  quit:s<86400  set s=s-86400,d=d+1
 quit d_","_s
