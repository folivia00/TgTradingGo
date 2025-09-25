(function(){
  const tg = window.Telegram?.WebApp; if (tg) tg.expand();
  function hdrs(){ const h={}; if(tg?.initData) h['X-TG-Init-Data']=tg.initData; return h }
  const $ = (q)=> document.querySelector(q);

  const chart = LightweightCharts.createChart($('#chart'), { layout:{background:{type:0}, textColor:'#e6edf3'}, grid:{vertLines:{color:'#1f2837'}, horzLines:{color:'#1f2837'}}, timeScale:{timeVisible:true, secondsVisible:false} });
  const series = chart.addCandlestickSeries();
  series._markers = [];
  let modeInitialized = false;
  const current = { symbol: $('#symbol')?.value || 'BTCUSDT', tf: '1m', mode: $('#bt-mode')?.value || 'spot' };

  function setCurrent(sym, tf, mode){
    if(sym) current.symbol = sym;
    if(tf) current.tf = tf;
    if(mode) current.mode = mode;
  }

  async function loadHistory(){
    const sym = $('#symbol').value; const tf = activeTF();
    const to = new Date(); const from = new Date(to.getTime() - 24*60*60*1000);
    const mode = $('#bt-mode')?.value || current.mode || 'spot';
    setCurrent(sym, tf, mode);
    series.setData([]);
    series._markers = [];
    series.setMarkers([]);
    const url = `/api/history?symbol=${sym}&tf=${tf}&mode=${mode}&from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`;
    const res = await fetch(url, { headers: hdrs() }); const arr = await res.json();
    series.setData(arr.map(k=>({ time: Math.floor(k.t/1000), open:k.o, high:k.h, low:k.l, close:k.c })));
  }

  // SSE
  const out = $('#stream');
  const es = new EventSource('/sse');
  es.onmessage = (e)=>{ try{ const j=JSON.parse(e.data);
    const ds = j?.data?.symbol; const dtf = j?.data?.tf;
    const mine = (!ds || ds===current.symbol) && (!dtf || dtf===current.tf);
    if(!mine) return;
    if(j.type==='candle'&&j.data){ series.update({ time: Math.floor(j.data.t/1000), open:j.data.o,high:j.data.h,low:j.data.l,close:j.data.c }); }
    if(j.type==='trade'&&j.data){
      const d=j.data; const ts=Math.floor(new Date(d.ts).getTime()/1000);
      let marker=null;
      if(d.side==='long' || d.side==='buy'){
        marker={ time:ts, position:'belowBar', color:'#26a69a', shape:'arrowUp', text:(d.note||d.event||'BUY') };
      } else if(d.side==='short' || d.side==='sell'){
        marker={ time:ts, position:'aboveBar', color:'#ef5350', shape:'arrowDown', text:(d.note||d.event||'SELL') };
      } else {
        const color=(typeof d.pnl==='number' && d.pnl>=0)?'#42a5f5':'#ff7043';
        const txt = d.note || (typeof d.pnl==='number'?`PnL ${d.pnl.toFixed(2)}`:(d.event||'EXIT'));
        marker={ time:ts, position:'aboveBar', color, shape:'circle', text:txt };
      }
      if(marker){
        if(!Array.isArray(series._markers)){ series._markers = []; }
        series._markers.push(marker);
        series.setMarkers(series._markers.slice(-200));
      }
    }
    out.prepend((JSON.stringify(j,null,2)+"\n")); }catch{ out.prepend(e.data+"\n") } };

  // control buttons
  $('#feed-random').onclick = ()=> switchFeed('random');
  $('#feed-rest').onclick   = ()=> switchFeed('rest');
  $('#save-state').onclick  = ()=> post('/api/ctrl/save_state');
  $('#load-state').onclick  = ()=> post('/api/ctrl/load_state').then(loadStatus);
  $('#reset-state').onclick = ()=> post('/api/ctrl/reset_state').then(loadStatus);
  $('#load').onclick        = loadHistory;
  $('#symbol').addEventListener('change', async ()=>{
    const sym = $('#symbol').value; const tf = activeTF(); const mode = $('#bt-mode')?.value || current.mode;
    try{ await post('/api/ctrl/set_symbol', {symbol:sym, tf, mode}); }catch(err){ console.error(err); }
    await loadStatus();
    await loadHistory();
  });
  $('#ping').onclick        = async()=>{ const r=await fetch('/api/ping',{headers:hdrs()}); alert('Ping '+r.status) };

  // -------- Backtest UI ----------
  function toRFC3339Local(val){
    // <input type=datetime-local> не содержит зону; считаем это локальным и конвертим в ISO UTC
    if(!val) return new Date().toISOString();
    const d = new Date(val);
    return new Date(d.getTime() - d.getTimezoneOffset()*60000).toISOString();
  }
  $('#bt-run').onclick = async ()=>{
    const sym = $('#symbol').value;
    const tf  = activeTF();
    const from = toRFC3339Local($('#bt-from').value);
    const to   = toRFC3339Local($('#bt-to').value);
    const eq = parseFloat($('#bt-eq').value||'10000');
    const lev = parseFloat($('#bt-lev').value||'1');
    const strat = $('#bt-strat').value;
    let args = {}; try{ args = JSON.parse($('#bt-args').value||'{}'); }catch{}
    let fees = {}; try{ fees = JSON.parse($('#bt-fees').value||'{}'); }catch{}
    const exchange = ($('#bt-mode')?.value || 'spot');
    const body = { symbol:sym, tf, from, to, initialEquity:eq, leverage:lev, slippageBps:0, strategy:strat, args, fees, exchange };
    const res = await fetch('/api/backtest', { method:'POST', headers:{'Content-Type':'application/json', ...hdrs()}, body: JSON.stringify(body) });
    if(!res.ok){ alert('Backtest error'); return }
    const j = await res.json();
    $('#bt-summary').textContent = `PNL=${j.summary.pnl.toFixed(2)} | Trades=${j.summary.trades} | WinRate=${(j.summary.winRate*100).toFixed(1)}% | PF=${j.summary.profitFactor.toFixed(2)} | MaxDD=${j.summary.maxDD.toFixed(2)}%`;
    const a = $('#bt-zip'); a.href = j.artifacts.zip; a.style.display='inline-block';
    msg('Бэктест готов — ZIP доступен');
  };

  // init значения дат: последние 24 часа
  (function initBacktestDates(){
    const to = new Date();
    const from = new Date(to.getTime()-24*60*60*1000);
    const pad = (n)=> String(n).padStart(2,'0');
    const fmt = (d)=> `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
    $('#bt-from').value = fmt(from);
    $('#bt-to').value   = fmt(to);
  })();

  function activeTF(){ const el=document.querySelector('.chip.active'); return el? el.dataset.tf : '1m' }
  document.querySelectorAll('.chip').forEach(el=>{ el.onclick=async()=>{
    document.querySelectorAll('.chip').forEach(c=>c.classList.remove('active'));
    el.classList.add('active');
    const sym = $('#symbol').value; const tf = activeTF(); const mode = $('#bt-mode')?.value || current.mode;
    try{ await post('/api/ctrl/set_symbol', {symbol:sym, tf, mode}); }catch(err){ console.error(err); }
    await loadStatus();
    await loadHistory();
  }; });
  document.querySelector('.chip[data-tf="1m"]').classList.add('active');

  function updateFeedButtons(feed){
    const randomBtn = $('#feed-random');
    const restBtn = $('#feed-rest');
    randomBtn.classList.toggle('active', feed === 'random');
    restBtn.classList.toggle('active', feed === 'rest');
  }

  async function switchFeed(feed){
    const randomBtn = $('#feed-random');
    const restBtn = $('#feed-rest');
    randomBtn.disabled = true;
    restBtn.disabled = true;
    try{
      await post('/api/ctrl/switch_feed', {feed});
      updateFeedButtons(feed);
      await loadStatus();
      await loadHistory();
      msg(`Фид переключен: ${feed}`);
    } finally {
      randomBtn.disabled = false;
      restBtn.disabled = false;
    }
  }
  function post(url, body){ return fetch(url,{ method:'POST', headers:{'Content-Type':'application/json', ...hdrs()}, body: body? JSON.stringify(body): null }).then(r=>{ if(!r.ok) throw new Error('request failed'); return r.json(); }) }

  async function loadStatus(){
    const r = await fetch('/api/status',{headers:hdrs()});
    if(!r.ok) return;
    const s = await r.json();
    $('#status').textContent = `mode=${s.mode} | ${s.symbol}/${s.tf} | feed=${s.feed} | exchange=${s.exchange||'spot'} | equity=${(s.equity||0).toFixed(2)}`;
    if(s.symbol && s.tf){ setCurrent(s.symbol, s.tf); }
    if(s.exchange){ current.mode = s.exchange; }
    if(!modeInitialized){
      const sel = $('#bt-mode'); if(sel && s.exchange){ sel.value = s.exchange; }
      modeInitialized = true;
    }
    updateFeedButtons(s.feed);
  }

  function msg(t){ const el=$('#aria-msg'); el.textContent=t; setTimeout(()=>el.textContent='Привет! Я помогу 🌸', 4000) }

  // init
  const modeSel = $('#bt-mode');
  if(modeSel){ modeSel.addEventListener('change', async ()=>{
    const sym = $('#symbol').value; const tf = activeTF(); const mode = modeSel.value || 'spot';
    try{ await post('/api/ctrl/set_symbol', {symbol:sym, tf, mode}); }catch(err){ console.error(err); }
    await loadStatus();
    await loadHistory();
  }); }

  loadHistory(); loadStatus();
})();
