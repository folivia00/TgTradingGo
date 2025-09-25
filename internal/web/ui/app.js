(function(){
  const tg = window.Telegram?.WebApp; if (tg) tg.expand();
  function hdrs(){ const h={}; if(tg?.initData) h['X-TG-Init-Data']=tg.initData; return h }
  const $ = (q)=> document.querySelector(q);

  const chart = LightweightCharts.createChart($('#chart'), { layout:{background:{type:0}, textColor:'#e6edf3'}, grid:{vertLines:{color:'#1f2837'}, horzLines:{color:'#1f2837'}}, timeScale:{timeVisible:true, secondsVisible:false} });
  const series = chart.addCandlestickSeries();

  async function loadHistory(){
    const sym = $('#symbol').value; const tf = activeTF();
    const to = new Date(); const from = new Date(to.getTime() - 6*60*60*1000);
    const url = `/api/history?symbol=${sym}&tf=${tf}&from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`;
    const res = await fetch(url, { headers: hdrs() }); const arr = await res.json();
    series.setData(arr.map(k=>({ time: Math.floor(k.t/1000), open:k.o, high:k.h, low:k.l, close:k.c })));
  }

  // SSE
  const out = $('#stream');
  const es = new EventSource('/sse');
  es.onmessage = (e)=>{ try{ const j=JSON.parse(e.data); if(j.type==='candle'&&j.data){ series.update({ time: Math.floor(j.data.t/1000), open:j.data.o,high:j.data.h,low:j.data.l,close:j.data.c }); } out.prepend((JSON.stringify(j,null,2)+"\n")); }catch{ out.prepend(e.data+"\n") } };

  // control buttons
  $('#feed-random').onclick = ()=> switchFeed('random');
  $('#feed-rest').onclick   = ()=> switchFeed('rest');
  $('#save-state').onclick  = ()=> post('/api/ctrl/save_state');
  $('#load-state').onclick  = ()=> post('/api/ctrl/load_state').then(loadStatus);
  $('#reset-state').onclick = ()=> post('/api/ctrl/reset_state').then(loadStatus);
  $('#load').onclick        = loadHistory;
  $('#ping').onclick        = async()=>{ const r=await fetch('/api/ping',{headers:hdrs()}); alert('Ping '+r.status) };

  function activeTF(){ const el=document.querySelector('.chip.active'); return el? el.dataset.tf : '1m' }
  document.querySelectorAll('.chip').forEach(el=>{ el.onclick=()=>{ document.querySelectorAll('.chip').forEach(c=>c.classList.remove('active')); el.classList.add('active'); loadHistory(); }; });
  document.querySelector('.chip[data-tf="1m"]').classList.add('active');

  async function switchFeed(feed){ await post('/api/ctrl/switch_feed', {feed}); msg(`Фид переключен: ${feed}`) }
  function post(url, body){ return fetch(url,{ method:'POST', headers:{'Content-Type':'application/json', ...hdrs()}, body: body? JSON.stringify(body): null }).then(r=>{ if(!r.ok) throw new Error('request failed'); return r.json(); }) }

  async function loadStatus(){ const r=await fetch('/api/status',{headers:hdrs()}); if(!r.ok) return; const s=await r.json(); $('#status').textContent = `mode=${s.mode} | ${s.symbol}/${s.tf} | feed=${s.feed} | equity=${(s.equity||0).toFixed(2)}` }

  function msg(t){ const el=$('#aria-msg'); el.textContent=t; setTimeout(()=>el.textContent='Привет! Я помогу 🌸', 4000) }

  // init
  loadHistory(); loadStatus();
})();
