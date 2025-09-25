(function(){
  const tg = window.Telegram?.WebApp; if (tg) tg.expand();
  function hdrs(){ const h={}; if(tg?.initData) h['X-TG-Init-Data']=tg.initData; return h }
  const $ = (q)=> document.querySelector(q);
  const ariaImg = "data:image/png;base64," +
    "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";
  const ariaEl = document.getElementById('aria-img');
  if (ariaEl) ariaEl.src = ariaImg;

  // chart
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
