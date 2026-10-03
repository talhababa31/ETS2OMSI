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
  <div class="class-row"><label for="classSelect">Araç sınıfı</label><select id="classSelect">${classOptions(v)}</select><small id="classBasis">${esc(classBasisText(v))}</small></div>
  <div class="detail-actions"><button id="previewBtn" class="secondary" ${!(v.models||[]).length?'disabled':''}>3D Önizle</button><button id="extractOne" class="secondary">Aracı Ayır</button><button id="selectThis" class="ghost" ${st!=='ready'?'disabled':''}>${selected.has(v.id)?'Seçimi Kaldır':'Dönüşüme Ekle'}</button></div>
  <div class="soft-label">MODEL SET</div><div class="tree">${esc(models)}</div>
  ${optional.length?`<div class="soft-label" style="margin-top:12px">OMSI İÇİN ZORUNLU OLMAYAN ETS2 HELPERS</div><ul class="warning-list">${optional.map(x=>`<li>${esc(x)}</li>`).join('')}</ul>`:''}
  ${(v.blocking||[]).length?`<div class="soft-label" style="margin-top:12px">BLOCKING</div><ul class="warning-list">${v.blocking.map(x=>`<li>${esc(x)}</li>`).join('')}</ul>`:''}`;
  $('#classSelect').onchange=e=>{if(e.target.value)classOverride[v.id]=e.target.value;else delete classOverride[v.id];$('#classBasis').textContent=classBasisText(v)};$('#previewBtn').onclick=()=>loadPreview(v,'final');$('#extractOne').onclick=()=>extractIDs([v.id]);$('#selectThis').onclick=()=>{toggle(v.id,!selected.has(v.id));renderVehicles();renderDetails()}
}

async function extractIDs(ids){
  clearError();const out=$('#outputRoot').value.trim()||'extracted';$('#conversionPanel').classList.remove('hidden');$('#conversionState').textContent='Ayırılıyor…';$('#conversionState').className='state-pill info';$('#conversionList').innerHTML='<div class="conversion-item"><div><b>SCS araç paketi hazırlanıyor…</b><small>Yalnız seçilen arabanın package-local dosyaları çıkarılıyor.</small></div></div>';
  try{const d=await post('/api/extract',{ids,output_root:out});$('#conversionState').textContent='Ayrıldı';$('#conversionState').className='state-pill ok';$('#conversionList').innerHTML=(d.items||[]).map(x=>`<div class="conversion-item"><div><b>${esc(x.name||x.id)}</b><small>${x.error?esc(x.error):`${esc(x.output)} · ${x.file_count} dosya`}</small></div><span class="state-pill ${x.error?'bad':'ok'}">${x.error?'Hata':'Hazır'}</span></div>`).join('');$('#conversionPanel').scrollIntoView({behavior:'smooth',block:'center'})}catch(e){showError(e);$('#conversionState').textContent='Hata';$('#conversionState').className='state-pill bad'}
}

async function convertSelected(){
  if(!selected.size)return;clearError();saveOutput();const ids=[...selected],out=$('#outputRoot').value.trim()||'output';$('#conversionPanel').classList.remove('hidden');$('#conversionState').textContent='Dönüştürülüyor…';$('#conversionState').className='state-pill info';$('#conversionList').innerHTML=ids.map(id=>{const v=(report.vehicles||[]).find(x=>x.id===id);return `<div class="conversion-item"><div><b>${esc(v?.display_name||id)}</b><small>PMD/PMG çözülüyor → OMSI O3D hazırlanıyor…</small></div><span class="state-pill info">Bekliyor</span></div>`}).join('');$('#conversionPanel').scrollIntoView({behavior:'smooth',block:'center'});$('#convertBtn').disabled=true;$('#convertBtn').textContent='Dönüştürülüyor…';
  try{const classes={};for(const id of ids)if(classOverride[id])classes[id]=classOverride[id];const d=await post('/api/convert',{ids,output_root:out,classes});renderConversions(d.items||[])}catch(e){showError(e);$('#conversionState').textContent='Hata';$('#conversionState').className='state-pill bad'}finally{$('#convertBtn').disabled=false;$('#convertBtn').textContent="OMSI'ye Dönüştür →"}
}
function renderConversions(items){
  const failed=items.filter(x=>x.error||x.report?.status==='fail').length;$('#conversionState').textContent=failed?`${failed} başarısız`:'Tamamlandı';$('#conversionState').className=`state-pill ${failed?'warn':'ok'}`;
  $('#conversionList').innerHTML=items.map(x=>{const r=x.report||{},bad=!!(x.error||r.status==='fail'),warnings=(r.warnings||[]).slice(0,4);let info='';if(bad){info=`Aşama: ${r.stage||'conversion'} · ${x.error||((r.errors||[]).join(' · '))||'Bilinmeyen hata'}`}else{const val=r.validation||{},tex=r.textures||{};const a=r.automation||{},phys=(a.vehicle_class?` · Sınıf ${classLabel(a.vehicle_class)}${a.class_basis?` (${a.class_basis})`:''}`:'')+(a.estimated_mass_t?` · Fizik ${a.estimated_mass_t} t`:'');info=`${r.output} · ${r.models?.length||0} model · ${tex.copied||0} texture · Exact ${tex.exact_resolved||0}${phys} · Yön ${val.orientation_ok?'OK':'FAIL'} · Zemin ${val.ground_ok?'OK':'WARN'} · Teker ${val.wheel_meshes||0}/4 · O3D ${val.o3d_readback_ok?'OK':'?'}${warnings.length?` · ${warnings.length} uyarı`:''}`}
    return `<div class="conversion-item"><div><b>${esc(r.name||x.name||x.id)}</b><small>${esc(info)}</small>${warnings.length?`<small class="warn-text">${warnings.map(esc).join(' · ')}</small>`:''}</div><span class="state-pill ${bad?'bad':r.status==='warn'?'warn':'ok'}">${bad?'FAILED':r.status==='warn'?'UYARI':'OMSI READY'}</span></div>`}).join('')||'<div class="muted">Sonuç yok.</div>'
}

async function loadPreview(v,mode='source'){
  previewMode=mode;clearError();$('#previewPanel').classList.remove('hidden');$('#previewTitle').textContent=v.display_name+' · '+(mode==='final'?'OMSI FINAL':'SOURCE');$('#previewLoading').classList.remove('hidden');$('#previewInfo').innerHTML='';$('#previewPanel').scrollIntoView({behavior:'smooth',block:'center'});
  try{const d=await api('/api/preview?id='+encodeURIComponent(v.id)+'&mode='+mode);$('#previewLoading').classList.add('hidden');$('#previewInfo').innerHTML=`<b>${Number(d.triangles_original||0).toLocaleString()}</b> source tris<br><b>${Number(d.triangles_preview||0).toLocaleString()}</b> preview tris<br>${d.wheel_visuals||0} ek wheel visual<br><br><b>${Number(d.length||0).toFixed(2)} × ${Number(d.width||0).toFixed(2)} × ${Number(d.height||0).toFixed(2)} m</b><br>Uzunluk × genişlik × yükseklik<br><br>${(d.materials||[]).length} material · ${(d.locators||[]).length} locator<br><br><span class="muted">${esc(d.note||'')}</span>`;previewRenderer?.destroy?.();previewRenderer=createRenderer($('#previewCanvas'),d);renderMaterialDiag(v,d);if(d.class){detectedClass[v.id]={class:d.class,basis:d.class_basis};$('#previewInfo').insertAdjacentHTML('afterbegin',`<b>${esc(classLabel(d.class))}</b> araç sınıfı<br><span class="muted">${esc(d.class_basis||'')}${classOverride[v.id]?' · elle seçilen: '+esc(classLabel(classOverride[v.id])):''}</span><br><br>`);if(activeVehicle()?.id===v.id&&$('#classSelect'))$('#classSelect').innerHTML=classOptions(v)}}catch(e){$('#previewLoading').classList.add('hidden');showError(e);$('#previewInfo').textContent=e.message}
}


$('#browseBtn').onclick=async()=>{clearError();try{const x=await api('/api/pick');if(x.path)$('#pathInput').value=x.path}catch(e){if(!String(e.message).toLowerCase().includes('cancel'))showError(e)}};
$('#pickOutput').onclick=async()=>{try{const x=await api('/api/pick-folder?title='+encodeURIComponent('Output klasörü seç'));if(x.path){$('#outputRoot').value=x.path;saveOutput()}}catch(e){if(!String(e.message).toLowerCase().includes('cancel'))showError(e)}};
$('#scanBtn').onclick=scan;$('#pathInput').onkeydown=e=>{if(e.key==='Enter')scan()};$('#searchInput').oninput=renderVehicles;$('#statusFilter').onchange=renderVehicles;
$('#selectReadyBtn').onclick=()=>{for(const v of filteredVehicles())if(v.readiness==='ready')selected.add(v.id);renderVehicles();updateSelected()};$('#clearBtn').onclick=()=>{selected.clear();renderVehicles();updateSelected()};$('#extractBtn').onclick=()=>extractIDs([...selected]);$('#convertBtn').onclick=convertSelected;
$('#previewSource').onclick=()=>{const v=activeVehicle();if(v)loadPreview(v,'source')};$('#previewFinal').onclick=()=>{const v=activeVehicle();if(v)loadPreview(v,'final')};$('#closePreview').onclick=()=>{$('#previewPanel').classList.add('hidden');previewRenderer?.destroy?.()};
const dz=$('#dropZone');['dragenter','dragover'].forEach(ev=>dz.addEventListener(ev,e=>{e.preventDefault();dz.classList.add('drag')}));['dragleave','drop'].forEach(ev=>dz.addEventListener(ev,e=>{e.preventDefault();dz.classList.remove('drag')}));dz.ondrop=e=>{const f=e.dataTransfer.files?.[0];if(f?.path)$('#pathInput').value=f.path;else showError(new Error('Dosya yolunu almak için SCS Dosyası Seç düğmesini kullan.'))};
$('#outputRoot').onchange=saveOutput;$('#themeToggle').onclick=()=>{document.documentElement.classList.toggle('dark');localStorage.setItem('theme',document.documentElement.classList.contains('dark')?'dark':'light')};if(localStorage.getItem('theme')==='dark'||(!localStorage.getItem('theme')&&matchMedia('(prefers-color-scheme:dark)').matches))document.documentElement.classList.add('dark');init();

(function(){const q=document.getElementById('quitBtn');if(!q)return;q.addEventListener('click',async()=>{if(!confirm('ETS2OMSI kapatılsın mı?'))return;try{await fetch('/api/quit',{method:'POST'})}catch(e){}document.body.innerHTML='<div style="padding:40px;font:16px sans-serif;color:#ccc;background:#111;min-height:100vh">ETS2OMSI kapatıldı. Bu sekmeyi kapatabilirsin.</div>'})})();

function renderMaterialDiag(v,d){
  const box=$('#materialDiag');if(!box)return;const mats=d.materials||[];
  const icon=s=>s==='ok'?'<span class="md ok">✔ paketten</span>':s==='missing'?'<span class="md bad">✖ pakette yok</span>':'<span class="md warn">⚠ yedek</span>';
  const counts={ok:0,missing:0,fallback:0};mats.forEach(m=>{counts[m.status||'fallback']=(counts[m.status||'fallback']||0)+1});
  box.innerHTML=`<div class="md-head"><b>Materyaller</b><span>${counts.ok} paketten · ${counts.missing} pakette yok · ${counts.fallback} texture'sız</span><button id="saveDiag" class="mini">Teşhis dosyasını kaydet</button></div>
  <div class="md-list">${mats.map((m,i)=>`<div class="md-row" data-i="${i}"><div><b>${esc(m.model||'')}</b> · ${esc(m.name||'')} <small>${esc(m.class||'')}</small></div><div>${icon(m.status)}</div><div class="md-reason">${esc(m.reason||'')}${m.image?`<br><small>${esc(m.image)}</small>`:m.ref?`<br><small>${esc(m.ref)}</small>`:''}</div></div>`).join('')}</div>`;
  box.querySelectorAll('.md-row').forEach(r=>r.onclick=()=>{box.querySelectorAll('.md-row').forEach(x=>x.classList.remove('sel'));r.classList.add('sel');previewRenderer?.setHighlight?.(Number(r.dataset.i))});
  $('#saveDiag').onclick=()=>{const data={app:'ETS2OMSI',vehicle:{id:v.id,name:v.display_name,models:v.models,looks:v.looks,variants:v.variants},mode:d.mode,dimensions:[d.length,d.width,d.height],materials:mats};const blob=new Blob([JSON.stringify(data,null,2)],{type:'application/json'});const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download='ETS2OMSI_teshis_'+String(v.id).replace(/[^a-z0-9_.-]/gi,'_')+'.json';document.body.appendChild(a);a.click();setTimeout(()=>{URL.revokeObjectURL(a.href);a.remove()},1000)};
}

var classList=[],classOverride={},detectedClass={};
function classLabel(id){return (classList.find(c=>c.id===id)||{}).label||id}
function classOptions(v){const det=detectedClass[v.id];const auto=det?`Otomatik — ${classLabel(det.class)}`:'Otomatik (önizlemede/dönüşümde bulunur)';return `<option value="">${esc(auto)}</option>`+classList.map(c=>`<option value="${esc(c.id)}" ${classOverride[v.id]===c.id?'selected':''}>${esc(c.label)}</option>`).join('')}
function classBasisText(v){if(classOverride[v.id])return 'Elle seçildi';const det=detectedClass[v.id];return det?`Algılama: ${det.basis||''}`:''}
api('/api/classes').then(x=>{classList=x||[];if(activeVehicle()&&$('#classSelect'))$('#classSelect').innerHTML=classOptions(activeVehicle())}).catch(()=>{});
