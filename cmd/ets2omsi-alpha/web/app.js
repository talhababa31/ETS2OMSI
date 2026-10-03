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

var exportRun=null;
async function convertSelected(){
  if(!selected.size||exportRun)return;clearError();saveOutput();
  const ids=[...selected],out=$('#outputRoot').value.trim()||'output';
  const classes={};for(const id of ids)if(classOverride[id])classes[id]=classOverride[id];
  const colors=selectedColors.length?selectedColors:['none'];
  const run={ids,out,items:[],cancel:false,start:Date.now()};exportRun=run;
  $('#conversionPanel').classList.remove('hidden');$('#exportProgress').classList.remove('hidden');$('#exportCancel').classList.remove('hidden');$('#exportSummary').classList.add('hidden');
  $('#conversionList').innerHTML='';$('#conversionState').textContent='Dönüştürülüyor…';$('#conversionState').className='state-pill info';
  $('#exportSub').textContent=`${ids.length} araç · ${colors[0]==='none'?'sadece orijinal renk':`orijinal + ${colors.length} renk`} · çıktı: ${out}`;
  $('#conversionPanel').scrollIntoView({behavior:'smooth',block:'start'});$('#convertBtn').disabled=true;$('#convertBtn').textContent='Dönüştürülüyor…';
  for(let n=0;n<ids.length;n++){
    if(run.cancel)break;const id=ids[n],v=(report.vehicles||[]).find(x=>x.id===id);
    updateProgress(run,n,v?.display_name||id);
    let item;try{const d=await post('/api/convert',{ids:[id],output_root:out,classes,colors});item=(d.items||[])[0]||{id,error:'boş yanıt'}}catch(e){item={id,name:v?.display_name,error:e.message}}
    run.items.push(item);$('#conversionList').insertAdjacentHTML('beforeend',exportCard(item,run.items.length-1));bindCard(run.items.length-1);
  }
  updateProgress(run,run.items.length,'');$('#exportProgress').classList.add('hidden');$('#exportCancel').classList.add('hidden');
  renderExportSummary(run);exportRun=null;$('#convertBtn').disabled=false;$('#convertBtn').textContent="OMSI'ye Dönüştür →";
}
function updateProgress(run,done,name){
  const total=run.ids.length,pct=Math.round(done/total*100);$('#exportBar').style.width=pct+'%';
  let eta='';if(done>0&&done<total){const per=(Date.now()-run.start)/done,left=Math.round(per*(total-done)/1000);eta=` · kalan ~${left>90?Math.round(left/60)+' dk':left+' sn'}`}
  $('#exportNow').innerHTML=name?`<b>${done+1} / ${total}</b> · ${esc(name)} dönüştürülüyor…${eta}`:`<b>${done} / ${total}</b> tamamlandı`;
}
function itemState(x){const r=x.report||{};if(x.error||r.status==='fail')return'bad';return r.status==='warn'?'warn':'ok'}
function exportCard(x,i){
  const r=x.report||{},st=itemState(x),a=r.automation||{},val=r.validation||{},tex=r.textures||{};
  const cols=r.color_variants||[];
  const dots=`<span class="cdot orig" title="Orijinal"></span>`+cols.map(c=>c.hex?`<span class="cdot" style="background:${esc(c.hex)}" title="${esc(c.label)}"></span>`:`<span class="cdot look" title="${esc(c.label)}"></span>`).join('');
  const warns=r.warnings||[],errs=(r.errors||[]).concat(x.error?[x.error]:[]);
  const facts=st==='bad'?`<span class="fact bad">Aşama: ${esc(r.stage||'dönüşüm')}</span>`:[
    a.vehicle_class?`<span class="fact">${esc(classLabel(a.vehicle_class))}${a.class_basis?` <small>(${esc(a.class_basis)})</small>`:''}</span>`:'',
    `<span class="fact">${dots} ${cols.length+1} renk</span>`,
    `<span class="fact">Teker ${val.wheel_meshes||0}/4</span>`,
    `<span class="fact">${tex.copied||0} texture</span>`,
    a.estimated_mass_t?`<span class="fact">${a.estimated_mass_t} t</span>`:''].join('');
  const label={ok:'OMSI HAZIR',warn:'UYARILI',bad:'HATA'}[st];
  return `<div class="export-card ${st}" data-i="${i}"><div class="ec-head"><div><b>${esc(r.name||x.name||x.id)}</b><small>${esc(r.output||'')}</small></div><span class="state-pill ${st}">${label}</span></div>
  <div class="ec-facts">${facts}</div>
  ${errs.length?`<div class="ec-err">${errs.map(esc).join('<br>')}</div>`:''}
  ${warns.length?`<details class="ec-warn"><summary>${warns.length} uyarı</summary><ul>${warns.map(w=>`<li>${esc(w)}</li>`).join('')}</ul></details>`:''}
  <div class="ec-actions">${r.output?`<button class="mini" data-act="open">Klasörü aç</button>`:''}${(r.ailist_lines||[]).length?`<button class="mini" data-act="copy">ailists satırlarını kopyala (${r.ailist_lines.length})</button>`:''}</div></div>`;
}
function bindCard(i){const c=document.querySelector(`.export-card[data-i="${i}"]`);if(!c)return;const x=exportRunItems()[i];
  c.querySelectorAll('[data-act]').forEach(b=>b.onclick=()=>{const r=x.report||{};if(b.dataset.act==='open')openFolder(r.output);else copyText((r.ailist_lines||[]).join('\r\n'),b)})}
var lastExportItems=[];function exportRunItems(){return exportRun?exportRun.items:lastExportItems}
function renderExportSummary(run){
  lastExportItems=run.items;const s={ok:0,warn:0,bad:0};let vehicles=0;const lines=[];
  for(const x of run.items){s[itemState(x)]++;if(itemState(x)!=='bad'){vehicles+=1+((x.report||{}).color_variants||[]).length;lines.push(...((x.report||{}).ailist_lines||[]))}}
  const stopped=run.items.length<run.ids.length;
  $('#conversionState').textContent=stopped?'Durduruldu':s.bad?`${s.bad} hata`:'Tamamlandı';$('#conversionState').className='state-pill '+(s.bad||stopped?'warn':'ok');
  const box=$('#exportSummary');box.classList.remove('hidden');
  box.innerHTML=`<div class="es-stats"><div><b>${s.ok}</b><span>hazır</span></div><div><b>${s.warn}</b><span>uyarılı</span></div><div><b>${s.bad}</b><span>hata</span></div><div><b>${vehicles}</b><span>OMSI aracı (renkler dahil)</span></div></div>
  <div class="es-actions"><button class="secondary" id="esOpen">Çıktı klasörünü aç</button><button class="primary" id="esCopy" ${lines.length?'':'disabled'}>Tüm ailists satırlarını kopyala (${lines.length})</button></div>
  <p class="muted">Kopyaladığın satırları haritanın <b>ailists.cfg</b> dosyasındaki AI grubuna yapıştır; her renk ayrı bir satırdır.</p>`;
  $('#esOpen').onclick=()=>openFolder(run.out);$('#esCopy').onclick=e=>copyText(lines.join('\r\n'),e.target);
  run.items.forEach((_,i)=>bindCard(i));
}
async function openFolder(p){try{await post('/api/open-folder',{path:p})}catch(e){showError(e)}}
async function copyText(t,btn){try{await navigator.clipboard.writeText(t)}catch(e){const ta=document.createElement('textarea');ta.value=t;document.body.appendChild(ta);ta.select();document.execCommand('copy');ta.remove()}
  if(btn){const o=btn.textContent;btn.textContent='Kopyalandı ✓';setTimeout(()=>btn.textContent=o,1500)}}

async function loadPreview(v,mode='source',color=''){
  previewMode=mode;clearError();$('#previewPanel').classList.remove('hidden');$('#previewTitle').textContent=v.display_name+' · '+(mode==='final'?'OMSI FINAL':'SOURCE');$('#previewLoading').classList.remove('hidden');$('#previewInfo').innerHTML='';$('#previewPanel').scrollIntoView({behavior:'smooth',block:'center'});
  try{const d=await api('/api/preview?id='+encodeURIComponent(v.id)+'&mode='+mode+'&color='+encodeURIComponent(color||''));renderSwatches(v,d);$('#previewLoading').classList.add('hidden');$('#previewInfo').innerHTML=`<b>${Number(d.triangles_original||0).toLocaleString()}</b> source tris<br><b>${Number(d.triangles_preview||0).toLocaleString()}</b> preview tris<br>${d.wheel_visuals||0} ek wheel visual<br><br><b>${Number(d.length||0).toFixed(2)} × ${Number(d.width||0).toFixed(2)} × ${Number(d.height||0).toFixed(2)} m</b><br>Uzunluk × genişlik × yükseklik<br><br>${(d.materials||[]).length} material · ${(d.locators||[]).length} locator<br><br><span class="muted">${esc(d.note||'')}</span>`;previewRenderer?.destroy?.();previewRenderer=createRenderer($('#previewCanvas'),d);renderMaterialDiag(v,d);if(d.class){detectedClass[v.id]={class:d.class,basis:d.class_basis};$('#previewInfo').insertAdjacentHTML('afterbegin',`<b>${esc(classLabel(d.class))}</b> araç sınıfı<br><span class="muted">${esc(d.class_basis||'')}${classOverride[v.id]?' · elle seçilen: '+esc(classLabel(classOverride[v.id])):''}</span><br><br>`);if(activeVehicle()?.id===v.id&&$('#classSelect'))$('#classSelect').innerHTML=classOptions(v)}}catch(e){$('#previewLoading').classList.add('hidden');showError(e);$('#previewInfo').textContent=e.message}
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

var paletteColors=[],selectedColors=[];
function renderSwatches(v,d){const box=$('#colorSwatches');if(!box)return;const cols=[{id:'',label:'Orijinal'}].concat(d.colors||[]);
  box.innerHTML='<span class="sw-label">Renk</span>'+cols.map(c=>`<button class="sw ${((d.color||'')===c.id)?'on':''}" data-id="${esc(c.id)}" title="${esc(c.label)}">${c.hex?`<i style="background:${esc(c.hex)}"></i>`:c.id?'<i class="look"></i>':'<i class="orig"></i>'}<span>${esc(c.label)}</span></button>`).join('');
  box.querySelectorAll('.sw').forEach(b=>b.onclick=()=>loadPreview(v,previewMode,b.dataset.id))}
function renderColorSettings(){const box=$('#colorSettings');if(!box)return;
  box.innerHTML=paletteColors.map(c=>`<button class="chip ${selectedColors.includes(c.id)?'on':''}" data-id="${esc(c.id)}"><i style="background:${esc(c.hex)}"></i>${esc(c.label)}</button>`).join('');
  box.querySelectorAll('.chip').forEach(b=>b.onclick=()=>{const id=b.dataset.id;selectedColors=selectedColors.includes(id)?selectedColors.filter(x=>x!==id):selectedColors.concat(id);try{localStorage.setItem('ets2omsi.colors',JSON.stringify(selectedColors))}catch(e){};renderColorSettings()});
  const n=$('#colorCount');if(n)n.textContent=selectedColors.length?`Her araç için ${selectedColors.length+1} renk (orijinal + ${selectedColors.length}) çıkarılır. ETS2'deki ek renkler (Look) de eklenir.`:'Sadece orijinal renk (ve ETS2 ek renkleri) çıkarılır.'}
api('/api/colors').then(x=>{paletteColors=x.palette||[];let saved=null;try{saved=JSON.parse(localStorage.getItem('ets2omsi.colors')||'null')}catch(e){};selectedColors=Array.isArray(saved)?saved:(x.default||[]);renderColorSettings()}).catch(()=>{});

document.addEventListener('click',e=>{if(e.target&&e.target.id==='exportCancel'&&exportRun){exportRun.cancel=true;e.target.textContent='Durduruluyor…'}});
