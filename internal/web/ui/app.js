(function(){
  const tg = window.Telegram?.WebApp; if (tg) tg.expand();
  document.getElementById('who').textContent = tg ? `theme=${tg.colorScheme} | user=${tg.initDataUnsafe?.user?.id||'n/a'}` : 'Telegram SDK not found';

  const chart = LightweightCharts.createChart(document.getElementById('chart'), { layout:{background:{type:0}, textColor:'#e6edf3'}, grid:{vertLines:{color:'#1f2837'}, horzLines:{color:'#1f2837'}}, timeScale:{timeVisible:true, secondsVisible:false} });
  const candleSeries = chart.addCandlestickSeries();

  function hdrs(){ const h={}; if(tg?.initData) h['X-TG-Init-Data']=tg.initData; return h }
  const now = ()=> new Date();
  const toISO = (d)=> d.toISOString();

  async function loadHistory(){
    const sym = document.getElementById('symbol').value;
    const tf = document.getElementById('tf').value;
    const to = now();
    const from = new Date(to.getTime() - 6*60*60*1000);
    const url = `/api/history?symbol=${sym}&tf=${tf}&from=${encodeURIComponent(toISO(from))}&to=${encodeURIComponent(toISO(to))}`;
    const res = await fetch(url, { headers: hdrs() });
    const arr = await res.json();
    candleSeries.setData(arr.map(k=>({ time: Math.floor(k.t/1000), open:k.o, high:k.h, low:k.l, close:k.c })));
  }

  document.getElementById('load').addEventListener('click', loadHistory);
  document.getElementById('ping').addEventListener('click', async ()=>{ const r=await fetch('/api/ping',{headers:hdrs()}); alert('Ping '+r.status) });

  // SSE live
  const out = document.getElementById('stream');
  const es = new EventSource('/sse');
  es.onmessage = (e)=>{
    try{ const j = JSON.parse(e.data);
      if (j.type === 'candle' && j.data) candleSeries.update({ time: Math.floor(j.data.t/1000), open:j.data.o, high:j.data.h, low:j.data.l, close:j.data.c });
      out.textContent = JSON.stringify(j,null,2)+"\n"+out.textContent;
    } catch { out.textContent = e.data+"\n"+out.textContent; }
  };

  // Backtest
  document.getElementById('run').addEventListener('click', async ()=>{
    const sym = document.getElementById('symbol').value;
    const tf = document.getElementById('tf').value;
    const from = document.getElementById('from').value; // local time -> assume browser locale, but API expects RFC3339; <input type=datetime-local> lacks tz, так что юзнем now‑tz
    const to = document.getElementById('to').value;
    const eq = parseFloat(document.getElementById('eq').value||'10000');
    const lev = parseFloat(document.getElementById('lev').value||'1');
    const strat = document.getElementById('strat').value;
    let args = {}; try{ args = JSON.parse(document.getElementById('args').value||'{}') }catch{}
    let fees = {}; try{ fees = JSON.parse(document.getElementById('fees').value||'{}') }catch{}

    const toRFC = (s)=> s? new Date(s).toISOString() : new Date().toISOString();
    const body = { symbol:sym, tf, from: toRFC(from), to: toRFC(to), initialEquity:eq, leverage:lev, slippageBps:0, strategy:strat, args, fees };
    const res = await fetch('/api/backtest', { method:'POST', headers:{ 'Content-Type':'application/json', ...hdrs() }, body: JSON.stringify(body) });
    if(!res.ok){ alert('Backtest error'); return }
    const j = await res.json();
    document.getElementById('sum').textContent = JSON.stringify(j.summary);
    const a = document.getElementById('zip'); a.href = j.artifacts.zip; a.style.display='inline-block';
  });

  // init
  const to = new Date(); const from = new Date(to.getTime()-24*60*60*1000);
  document.getElementById('from').value = new Date(from.getTime()-from.getTimezoneOffset()*60000).toISOString().slice(0,16);
  document.getElementById('to').value   = new Date(to.getTime()-to.getTimezoneOffset()*60000).toISOString().slice(0,16);
  loadHistory();
})();

