(function(){
  const tg = window.Telegram?.WebApp;
  if (tg) {
    tg.expand();
    document.getElementById('who').textContent = `theme=${tg.colorScheme} | initDataUnsafe.user=${tg.initDataUnsafe?.user?.id || 'n/a'}`;
  } else {
    document.getElementById('who').textContent = 'Telegram SDK not found (open inside Telegram)';
  }

  // Ping
  document.getElementById('ping').addEventListener('click', async ()=>{
    const res = await fetch('/api/ping', { headers: hdrs() });
    const j = await res.json(); alert('Ping: '+JSON.stringify(j));
  });

  // SSE stream
  const es = new EventSource('/sse');
  const out = document.getElementById('stream');
  es.onmessage = (e)=>{
    try{ const j = JSON.parse(e.data); out.textContent = JSON.stringify(j,null,2)+"\n"+out.textContent; }
    catch{ out.textContent = e.data+"\n"+out.textContent; }
  };
  es.onerror = ()=>{ out.textContent = '[sse closed]\n'+out.textContent; };

  function hdrs(){
    const h = { };
    if (tg?.initData) h['X-TG-Init-Data'] = tg.initData; // сервер валидирует подпись
    return h;
  }
})();

