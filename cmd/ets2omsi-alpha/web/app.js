const $ = s => document.querySelector(s);
let project=null, report=null, selected=new Set(), active=null, previewRenderer=null, previewMode='source';
const label={ready:'Hazır',model_missing:'Model bekliyor',blocked:'Sorunlu'};
const cls={ready:'ok',model_missing:'warn',blocked:'bad'};

async function api(url,opts={}){const r=await fetch(url,opts);const t=await r.text();let d;try{d=JSON.parse(t)}catch{d={error:t}};if(!r.ok)throw new Error(d.error||t||`HTTP ${r.status}`);return d}
function post(url,obj){return api(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(obj)})}
function esc(s=''){return String(s).replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]))}
function clearError(){$('#errorBox').classList.add('hidden')}
function showError(e){$('#errorBox').textContent=e?.message||String(e);$('#errorBox').classList.remove('hidden');$('#errorBox').scrollIntoView({behavior:'smooth',block:'center'})}
function setScanning(on){$('#scanState').classList.toggle('hidden',!on);$('#scanBtn').disabled=on}
function saveOutput(){localStorage.setItem('v2Output',$('#outputRoot').value||'')}

async function init(){
  const out=localStorage.getItem('v2Output'); if(out)$('#outputRoot').value=out;
  try{const s=await api('/api/status');const b=$('#toolBadge');b.textContent=s.converterpix?.found?'3D Engine · hazır':'3D Engine · ilk dönüşümde otomatik kurulur';b.className=s.converterpix?.found?'badge':'badge neutral';if(!s.scs_helper?.found)$('#helperHint').textContent='Not: HashFS tipindeki yeni .SCS paketleri için SCS helper gerekebilir. ZIP tabanlı traffic paketlerinde gerekmez.'}catch(e){showError(e)}
}

async function scan(){
  clearError(); const path=$('#pathInput').value.trim(); if(!path)return showError(new Error('Önce traffic .SCS dosyasını seç.'));
  setScanning(true); $('#resultArea').classList.add('hidden');
  try{project=await post('/api/scan',{path});report=project.report;selected.clear();active=null;render()}
  catch(e){showError(e);if(String(e.message).toLowerCase().includes('helper'))$('#helperHint').classList.remove('hidden')}
  finally{setScanning(false)}
}
function render(){
  $('#resultArea').classList.remove('hidden');
  const vs=report.vehicles||[];
  $('#vehicleCount').textContent=vs.length;
  $('#readyCount').textContent=vs.filter(v=>v.readiness==='ready').length;
  $('#modelCount').textContent=vs.filter(v=>(v.models||[]).length>0).length;
  $('#fileCount').textContent=Number(report.file_count||0).toLocaleString();
  $('#packageName').textContent=report.package;
  $('#packageMeta').textContent=`${String(report.archive_kind).toUpperCase()} · ${Number(report.file_count||0).toLocaleString()} dosya · yalnız AI otomobiller`;
  $('#mountText').textContent='Yalnızca '+report.package+' kullanılacak. ETS2/base/DLC aranmayacak.';
  renderVehicles();renderDetails();updateSelected();
}
function filteredVehicles(){const q=$('#searchInput').value.trim().toLowerCase(),f=$('#statusFilter').value;return(report?.vehicles||[]).filter(v=>(!q||`${v.display_name} ${v.id}`.toLowerCase().includes(q))&&(f==='all'||v.readiness===f))}
function renderVehicles(){
  const h=$('#vehicleList');h.innerHTML='';const list=filteredVehicles();
  if(!list.length){h.innerHTML='<div class="empty-state"><div class="big-icon">⌁</div><h3>Araba bulunamadı</h3><p>Filtreyi temizle veya başka traffic paketi dene.</p></div>';return}
  for(const v of list){
    const st=v.readiness||((v.models||[]).length?'ready':'model_missing');const d=document.createElement('div');d.className='vehicle-item'+(active===v.id?' active':'');
    const lod=Math.max(0,(v.models||[]).length-1);
    d.innerHTML=`<input type="checkbox" ${selected.has(v.id)?'checked':''} ${st!=='ready'?'disabled':''}><div><div class="vehicle-name">${esc(v.display_name)}</div><div class="vehicle-sub">${v.counts.models||0} model · ${lod} LOD · ${(v.chassis||[]).length} chassis${v.counts.missing?` · ${v.counts.missing} opsiyonel eksik`:''}</div></div><span class="state-pill ${cls[st]||'warn'}">${esc(label[st]||st)}</span>`;
    const cb=d.querySelector('input');cb.onclick=e=>{e.stopPropagation();toggle(v.id,cb.checked)};d.onclick=()=>{active=v.id;renderVehicles();renderDetails()};h.appendChild(d)
  }
}
function toggle(id,on){on?selected.add(id):selected.delete(id);updateSelected()}
function updateSelected(){const n=selected.size;$('#selectedText').textContent=`${n} araba seçildi`;$('#convertBtn').disabled=!n;$('#extractBtn').disabled=!n}
function activeVehicle(){return(report?.vehicles||[]).find(v=>v.id===active)}
function renderDetails(){
  const h=$('#detailsPanel'),v=activeVehicle();if(!v){h.innerHTML='<div class="empty-state"><div class="big-icon">◇</div><h3>Bir araba seç</h3><p>Ana PMD, LOD ve package-local dosyaları burada göreceksin.</p></div>';return}
  const st=v.readiness||'blocked';const models=(v.models||[]).map((m,i)=>`${i===0?'MAIN':'LOD '+i}  ${m}`).join('\n')||'Model bulunamadı';
  const optional=(v.shared_requirements||[]).slice(0,8);
  h.innerHTML=`<div class="detail-title"><div><div class="soft-label">AI CAR · SCS ONLY</div><h3>${esc(v.display_name)}</h3><p class="muted">${esc(v.id)}</p></div><span class="state-pill ${cls[st]||'warn'}">${esc(label[st]||st)}</span></div>
  <div class="detail-grid"><div class="detail-metric"><b>${v.counts.models||0}</b><span>3D Models</span></div><div class="detail-metric"><b>${Math.max(0,(v.models||[]).length-1)}</b><span>LODs</span></div><div class="detail-metric"><b>${(v.chassis||[]).length}</b><span>Chassis</span></div><div class="detail-metric"><b>${v.counts.textures||0}</b><span>Texture refs</span></div><div class="detail-metric"><b>${v.counts.dependencies||0}</b><span>Linked files</span></div><div class="detail-metric"><b>${v.counts.missing||0}</b><span>Optional missing</span></div></div>
  <div class="detail-actions"><button id="previewBtn" class="secondary" ${!(v.models||[]).length?'disabled':''}>3D Önizle</button><button id="extractOne" class="secondary">Aracı Ayır</button><button id="selectThis" class="ghost" ${st!=='ready'?'disabled':''}>${selected.has(v.id)?'Seçimi Kaldır':'Dönüşüme Ekle'}</button></div>
  <div class="soft-label">MODEL SET</div><div class="tree">${esc(models)}</div>
  ${optional.length?`<div class="soft-label" style="margin-top:12px">OMSI İÇİN ZORUNLU OLMAYAN ETS2 HELPERS</div><ul class="warning-list">${optional.map(x=>`<li>${esc(x)}</li>`).join('')}</ul>`:''}
  ${(v.blocking||[]).length?`<div class="soft-label" style="margin-top:12px">BLOCKING</div><ul class="warning-list">${v.blocking.map(x=>`<li>${esc(x)}</li>`).join('')}</ul>`:''}`;
  $('#previewBtn').onclick=()=>loadPreview(v,'source');$('#extractOne').onclick=()=>extractIDs([v.id]);$('#selectThis').onclick=()=>{toggle(v.id,!selected.has(v.id));renderVehicles();renderDetails()}
}

async function extractIDs(ids){
  clearError();const out=$('#outputRoot').value.trim()||'extracted';$('#conversionPanel').classList.remove('hidden');$('#conversionState').textContent='Ayırılıyor…';$('#conversionState').className='state-pill info';$('#conversionList').innerHTML='<div class="conversion-item"><div><b>SCS araç paketi hazırlanıyor…</b><small>Yalnız seçilen arabanın package-local dosyaları çıkarılıyor.</small></div></div>';
  try{const d=await post('/api/extract',{ids,output_root:out});$('#conversionState').textContent='Ayrıldı';$('#conversionState').className='state-pill ok';$('#conversionList').innerHTML=(d.items||[]).map(x=>`<div class="conversion-item"><div><b>${esc(x.name||x.id)}</b><small>${x.error?esc(x.error):`${esc(x.output)} · ${x.file_count} dosya`}</small></div><span class="state-pill ${x.error?'bad':'ok'}">${x.error?'Hata':'Hazır'}</span></div>`).join('');$('#conversionPanel').scrollIntoView({behavior:'smooth',block:'center'})}catch(e){showError(e);$('#conversionState').textContent='Hata';$('#conversionState').className='state-pill bad'}
}

async function convertSelected(){
  if(!selected.size)return;clearError();saveOutput();const ids=[...selected],out=$('#outputRoot').value.trim()||'output';$('#conversionPanel').classList.remove('hidden');$('#conversionState').textContent='Dönüştürülüyor…';$('#conversionState').className='state-pill info';$('#conversionList').innerHTML=ids.map(id=>{const v=(report.vehicles||[]).find(x=>x.id===id);return `<div class="conversion-item"><div><b>${esc(v?.display_name||id)}</b><small>PMD/PMG çözülüyor → OMSI O3D hazırlanıyor…</small></div><span class="state-pill info">Bekliyor</span></div>`}).join('');$('#conversionPanel').scrollIntoView({behavior:'smooth',block:'center'});$('#convertBtn').disabled=true;$('#convertBtn').textContent='Dönüştürülüyor…';
  try{const d=await post('/api/convert',{ids,output_root:out});renderConversions(d.items||[])}catch(e){showError(e);$('#conversionState').textContent='Hata';$('#conversionState').className='state-pill bad'}finally{$('#convertBtn').disabled=false;$('#convertBtn').textContent="OMSI'ye Dönüştür →"}
}
function renderConversions(items){
  const failed=items.filter(x=>x.error||x.report?.status==='fail').length;$('#conversionState').textContent=failed?`${failed} başarısız`:'Tamamlandı';$('#conversionState').className=`state-pill ${failed?'warn':'ok'}`;
  $('#conversionList').innerHTML=items.map(x=>{const r=x.report||{},bad=!!(x.error||r.status==='fail'),warnings=(r.warnings||[]).slice(0,4);let info='';if(bad){info=`Aşama: ${r.stage||'conversion'} · ${x.error||((r.errors||[]).join(' · '))||'Bilinmeyen hata'}`}else{const val=r.validation||{},tex=r.textures||{};const a=r.automation||{},phys=a.estimated_mass_t?` · Fizik ${a.estimated_mass_t} t${a.physics_profile?` (${a.physics_profile})`:''}`:'';info=`${r.output} · ${r.models?.length||0} model · ${tex.copied||0} texture · Exact ${tex.exact_resolved||0}${phys} · Yön ${val.orientation_ok?'OK':'FAIL'} · Zemin ${val.ground_ok?'OK':'WARN'} · Teker ${val.wheel_meshes||0}/4 · O3D ${val.o3d_readback_ok?'OK':'?'}${warnings.length?` · ${warnings.length} uyarı`:''}`}
    return `<div class="conversion-item"><div><b>${esc(r.name||x.name||x.id)}</b><small>${esc(info)}</small>${warnings.length?`<small class="warn-text">${warnings.map(esc).join(' · ')}</small>`:''}</div><span class="state-pill ${bad?'bad':r.status==='warn'?'warn':'ok'}">${bad?'FAILED':r.status==='warn'?'UYARI':'OMSI READY'}</span></div>`}).join('')||'<div class="muted">Sonuç yok.</div>'
}

async function loadPreview(v,mode='source'){
  previewMode=mode;clearError();$('#previewPanel').classList.remove('hidden');$('#previewTitle').textContent=v.display_name+' · '+(mode==='final'?'OMSI FINAL':'SOURCE');$('#previewLoading').classList.remove('hidden');$('#previewInfo').innerHTML='';$('#previewPanel').scrollIntoView({behavior:'smooth',block:'center'});
  try{const d=await api('/api/preview?id='+encodeURIComponent(v.id)+'&mode='+mode);$('#previewLoading').classList.add('hidden');$('#previewInfo').innerHTML=`<b>${Number(d.triangles_original||0).toLocaleString()}</b> source tris<br><b>${Number(d.triangles_preview||0).toLocaleString()}</b> preview tris<br>${d.wheel_visuals||0} ek wheel visual<br><br><b>${Number(d.length||0).toFixed(2)} × ${Number(d.width||0).toFixed(2)} × ${Number(d.height||0).toFixed(2)} m</b><br>Uzunluk × genişlik × yükseklik<br><br>${(d.materials||[]).length} material · ${(d.locators||[]).length} locator<br><br><span class="muted">${esc(d.note||'')}</span>`;previewRenderer?.destroy?.();previewRenderer=createRenderer($('#previewCanvas'),d)}catch(e){$('#previewLoading').classList.add('hidden');showError(e);$('#previewInfo').textContent=e.message}
}

function createRenderer(canvas,d){const gl=canvas.getContext('webgl',{antialias:true,alpha:false});if(!gl)return{destroy(){}};const vs=`attribute vec3 p;attribute vec3 n;attribute vec3 c;uniform mat4 mvp;uniform mat4 rot;varying vec3 vc;varying float l;void main(){vec3 nn=normalize((rot*vec4(n,0.0)).xyz);l=max(.22,dot(nn,normalize(vec3(.35,.55,.75))));vc=c;gl_Position=mvp*vec4(p,1.0);}`;const fs=`precision mediump float;varying vec3 vc;varying float l;void main(){gl_FragColor=vec4(vc*l,1.0);}`;const pr=link(gl,vs,fs);gl.useProgram(pr);const pos=[],nor=[],col=[],mats=d.materials||[{color:[.6,.6,.65]}];for(let ti=0;ti<(d.triangle_materials||[]).length;ti++){const mi=d.triangle_materials[ti]||0,c=mats[mi]?.color||[.6,.6,.65];for(let k=0;k<3;k++){const vi=d.indices[ti*3+k];pos.push(d.positions[vi*3],d.positions[vi*3+1],d.positions[vi*3+2]);nor.push(d.normals[vi*3]??0,d.normals[vi*3+1]??0,d.normals[vi*3+2]??1);col.push(c[0],c[1],c[2])}}bindAttr(gl,pr,'p',pos);bindAttr(gl,pr,'n',nor);bindAttr(gl,pr,'c',col);gl.enable(gl.DEPTH_TEST);gl.disable(gl.CULL_FACE);gl.clearColor(.045,.05,.065,1);let yaw=-.72,pitch=.26,zoom=Math.max(5,Math.max(d.length||1,d.width||1,d.height||1)*1.45),drag=false,lx=0,ly=0,raf=0;canvas.onmousedown=e=>{drag=true;lx=e.clientX;ly=e.clientY};const up=()=>drag=false;window.addEventListener('mouseup',up);canvas.onmousemove=e=>{if(!drag)return;yaw+=(e.clientX-lx)*.009;pitch=Math.max(-1.2,Math.min(1.2,pitch+(e.clientY-ly)*.009));lx=e.clientX;ly=e.clientY};canvas.onwheel=e=>{e.preventDefault();zoom=Math.max(.6,zoom*(1+Math.sign(e.deltaY)*.08))};function draw(){resizeCanvas(canvas);gl.viewport(0,0,canvas.width,canvas.height);gl.clear(gl.COLOR_BUFFER_BIT|gl.DEPTH_BUFFER_BIT);const proj=perspective(42*Math.PI/180,canvas.width/canvas.height,.05,2000),view=lookAt([0,-zoom*.95,zoom*.43],[0,0,Math.max(.2,(d.height||1)*.35)],[0,0,1]),rot=mul(rotationZ(yaw),rotationX(pitch)),mvp=mul(proj,mul(view,rot));gl.uniformMatrix4fv(gl.getUniformLocation(pr,'mvp'),false,new Float32Array(mvp));gl.uniformMatrix4fv(gl.getUniformLocation(pr,'rot'),false,new Float32Array(rot));gl.drawArrays(gl.TRIANGLES,0,pos.length/3);raf=requestAnimationFrame(draw)}draw();return{destroy(){cancelAnimationFrame(raf);window.removeEventListener('mouseup',up);canvas.onmousedown=canvas.onmousemove=canvas.onwheel=null}}}
function shader(gl,t,src){const s=gl.createShader(t);gl.shaderSource(s,src);gl.compileShader(s);if(!gl.getShaderParameter(s,gl.COMPILE_STATUS))throw new Error(gl.getShaderInfoLog(s));return s}function link(gl,v,f){const p=gl.createProgram();gl.attachShader(p,shader(gl,gl.VERTEX_SHADER,v));gl.attachShader(p,shader(gl,gl.FRAGMENT_SHADER,f));gl.linkProgram(p);if(!gl.getProgramParameter(p,gl.LINK_STATUS))throw new Error(gl.getProgramInfoLog(p));return p}function bindAttr(gl,p,n,data){const b=gl.createBuffer();gl.bindBuffer(gl.ARRAY_BUFFER,b);gl.bufferData(gl.ARRAY_BUFFER,new Float32Array(data),gl.STATIC_DRAW);const a=gl.getAttribLocation(p,n);gl.enableVertexAttribArray(a);gl.vertexAttribPointer(a,3,gl.FLOAT,false,0,0)}function resizeCanvas(c){const q=devicePixelRatio||1,w=Math.floor(c.clientWidth*q),h=Math.floor(c.clientHeight*q);if(c.width!==w||c.height!==h){c.width=w;c.height=h}}function perspective(f,a,n,fa){const q=1/Math.tan(f/2),nf=1/(n-fa);return[q/a,0,0,0,0,q,0,0,0,0,(fa+n)*nf,-1,0,0,2*fa*n*nf,0]}function rotationX(a){const c=Math.cos(a),s=Math.sin(a);return[1,0,0,0,0,c,s,0,0,-s,c,0,0,0,0,1]}function rotationZ(a){const c=Math.cos(a),s=Math.sin(a);return[c,s,0,0,-s,c,0,0,0,0,1,0,0,0,0,1]}function mul(a,b){const o=new Array(16).fill(0);for(let c=0;c<4;c++)for(let r=0;r<4;r++)for(let k=0;k<4;k++)o[c*4+r]+=a[k*4+r]*b[c*4+k];return o}function lookAt(e,t,u){const z=norm(sub(e,t)),x=norm(cross(u,z)),y=cross(z,x);return[x[0],y[0],z[0],0,x[1],y[1],z[1],0,x[2],y[2],z[2],0,-dot(x,e),-dot(y,e),-dot(z,e),1]}function sub(a,b){return[a[0]-b[0],a[1]-b[1],a[2]-b[2]]}function dot(a,b){return a[0]*b[0]+a[1]*b[1]+a[2]*b[2]}function cross(a,b){return[a[1]*b[2]-a[2]*b[1],a[2]*b[0]-a[0]*b[2],a[0]*b[1]-a[1]*b[0]]}function norm(a){const l=Math.hypot(...a)||1;return a.map(x=>x/l)}

$('#browseBtn').onclick=async()=>{clearError();try{const x=await api('/api/pick');if(x.path)$('#pathInput').value=x.path}catch(e){if(!String(e.message).toLowerCase().includes('cancel'))showError(e)}};
$('#pickOutput').onclick=async()=>{try{const x=await api('/api/pick-folder?title='+encodeURIComponent('Output klasörü seç'));if(x.path){$('#outputRoot').value=x.path;saveOutput()}}catch(e){if(!String(e.message).toLowerCase().includes('cancel'))showError(e)}};
$('#scanBtn').onclick=scan;$('#pathInput').onkeydown=e=>{if(e.key==='Enter')scan()};$('#searchInput').oninput=renderVehicles;$('#statusFilter').onchange=renderVehicles;
$('#selectReadyBtn').onclick=()=>{for(const v of filteredVehicles())if(v.readiness==='ready')selected.add(v.id);renderVehicles();updateSelected()};$('#clearBtn').onclick=()=>{selected.clear();renderVehicles();updateSelected()};$('#extractBtn').onclick=()=>extractIDs([...selected]);$('#convertBtn').onclick=convertSelected;
$('#previewSource').onclick=()=>{const v=activeVehicle();if(v)loadPreview(v,'source')};$('#previewFinal').onclick=()=>{const v=activeVehicle();if(v)loadPreview(v,'final')};$('#closePreview').onclick=()=>{$('#previewPanel').classList.add('hidden');previewRenderer?.destroy?.()};
const dz=$('#dropZone');['dragenter','dragover'].forEach(ev=>dz.addEventListener(ev,e=>{e.preventDefault();dz.classList.add('drag')}));['dragleave','drop'].forEach(ev=>dz.addEventListener(ev,e=>{e.preventDefault();dz.classList.remove('drag')}));dz.ondrop=e=>{const f=e.dataTransfer.files?.[0];if(f?.path)$('#pathInput').value=f.path;else showError(new Error('Dosya yolunu almak için SCS Dosyası Seç düğmesini kullan.'))};
$('#outputRoot').onchange=saveOutput;$('#themeToggle').onclick=()=>{document.documentElement.classList.toggle('dark');localStorage.setItem('theme',document.documentElement.classList.contains('dark')?'dark':'light')};if(localStorage.getItem('theme')==='dark'||(!localStorage.getItem('theme')&&matchMedia('(prefers-color-scheme:dark)').matches))document.documentElement.classList.add('dark');init();

(function(){const q=document.getElementById('quitBtn');if(!q)return;q.addEventListener('click',async()=>{if(!confirm('ETS2OMSI kapatılsın mı?'))return;try{await fetch('/api/quit',{method:'POST'})}catch(e){}document.body.innerHTML='<div style="padding:40px;font:16px sans-serif;color:#ccc;background:#111;min-height:100vh">ETS2OMSI kapatıldı. Bu sekmeyi kapatabilirsin.</div>'})})();
