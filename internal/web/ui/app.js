(function(){
  const tg = window.Telegram?.WebApp; if (tg) tg.expand();
  document.getElementById('who').textContent = tg ? `theme=${tg.colorScheme} | user=${tg.initDataUnsafe?.user?.id||'n/a'}` : 'Telegram SDK not found';

  // Chart init
  const chart = LightweightCharts.createChart(document.getElementById('chart'), { layout:{background:{type:0}, textColor:'#e6edf3'}, grid:{vertLines:{color:'#1f2837'}, horzLines:{color:'#1f2837'}}, timeScale:{timeVisible:true, secondsVisible:false} });
  const candleSeries = chart.addCandlestickSeries();

  // Helpers
  const now = ()=> new Date();
  const toISO = (d)=> d.toISOString();
  function hdrs(){ const h={}; if(tg?.initData) h['X-TG-Init-Data']=tg.initData; return h }

  // Load history
  async function loadHistory(){
    const sym = document.getElementById('symbol').value;
    const tf = document.getElementById('tf').value;
    const to = now();
    const from = new Date(to.getTime() - 6*60*60*1000); // последние 6 часов
    const url = `/api/history?symbol=${sym}&tf=${tf}&from=${encodeURIComponent(toISO(from))}&to=${encodeURIComponent(toISO(to))}`;
    const res = await fetch(url, { headers: hdrs() });
    const arr = await res.json();
    const data = arr.map(k=>({ time: Math.floor(k.t/1000), open:k.o, high:k.h, low:k.l, close:k.c }));
    candleSeries.setData(data);
  }

  document.getElementById('load').addEventListener('click', loadHistory);
  document.getElementById('ping').addEventListener('click', async ()=>{ const r=await fetch('/api/ping',{headers:hdrs()}); alert('Ping '+r.status) });

  // SSE stream
  const out = document.getElementById('stream');
  const es = new EventSource('/sse');
  es.onmessage = (e)=>{
    try{ const j = JSON.parse(e.data);
      if (j.type === 'candle' && j.data) {
        // live update last bar
        candleSeries.update({ time: Math.floor(j.data.t/1000), open:j.data.o, high:j.data.h, low:j.data.l, close:j.data.c });
      }
      out.textContent = JSON.stringify(j,null,2)+"\n"+out.textContent;
    } catch { out.textContent = e.data+"\n"+out.textContent; }
  };
  es.onerror = ()=>{ out.textContent = '[sse closed]\n'+out.textContent };

  // авто‑подгрузка истории при старте
  loadHistory();
})();
