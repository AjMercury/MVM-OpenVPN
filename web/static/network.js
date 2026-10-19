'use strict';
const networkForm = document.getElementById('network-form');
if (networkForm) {
  const fa = document.documentElement.lang === 'fa';
  networkForm.addEventListener('submit', async event => {
    event.preventDefault();
    const errors = document.getElementById('network-errors'), button = networkForm.querySelector('button[type=submit]');
    errors.hidden = true; button.disabled = true;
    try {
      const values = Object.fromEntries(new FormData(networkForm));
      const spec = {router_id:values.router_id,source_id:values.source_id,out_interface_id:values.out_interface_id,nat_anchor:values.nat_anchor,forward_anchor:values.forward_anchor,allow_forward:values.allow_forward==='yes'};
      const response = await fetch(networkForm.action,{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':document.querySelector('meta[name="csrf-token"]').content},body:JSON.stringify({spec,idempotency_key:values.idempotency_key})});
      const data = await response.json();
      if (!response.ok) {
        const translations = {
          'select a current source range and an enabled interface from this router':'رنج یا اینترفیس انتخاب‌شده دیگر در این روتر موجود نیست؛ فهرست را دوباره دریافت کنید.',
          'firewall or NAT order changed; reload the router selections':'ترتیب قوانین NAT یا فایروال تغییر کرده است؛ فهرست روتر را دوباره دریافت کنید.',
          'permission denied':'برای تغییر NAT و فایروال به دسترسی مدیر روتر نیاز دارید.',
          'forward chain begins with a dynamic rule; NAT-only is available, but forwarding requires manual order review':'ابتدای زنجیره forward قانون پویا دارد؛ فقط NAT قابل تنظیم است و اجازه عبور به بررسی ترتیب قوانین نیاز دارد.',
          'a network job already exists for this source and output interface; review it before creating another':'برای این رنج و اینترفیس، عملیات شبکه از قبل وجود دارد؛ آن را در بخش عملیات بررسی کنید.'
        };
        const message=data.error?.message||String(response.status);
        throw new Error(fa ? (translations[message]||message) : message);
      }
      location.assign('/?page=job&id='+encodeURIComponent(data.id));
    } catch (error) {
      errors.textContent=(fa?'ذخیره انجام نشد: ':'Unable to save: ')+error.message;errors.hidden=false;errors.focus();
    } finally { button.disabled=false; }
  });
}
