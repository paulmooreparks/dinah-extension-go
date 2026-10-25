(function () {
 'use strict';
 let port, sequence=0, readyResolve, lastFocused=document.activeElement;
 const pending=new Map(), listeners=new Map(), focusTargets=new Map();
 const ready=new Promise(resolve=>{readyResolve=resolve;});
 // The host keeps one shared map per extension and workspace while any frame of the
 // extension lives; set() writes a key for the extension's other frames there,
 // and a 'shared' event brings each change they make. The map is not persisted.
 let shared={};
 const api=window.dinah={ready,theme:null,workspace:null,
  async call(action,input){await ready;return request('call',{action,input:input||{}});},
  on(name,fn){if(!listeners.has(name))listeners.set(name,new Set());listeners.get(name).add(fn);return()=>listeners.get(name)?.delete(fn);},
  commands(list){return ready.then(()=>request('commands',{commands:list}));},
  navigate(state){return ready.then(()=>request('navigate',{state}));},
  hostCommand(command,payload){let focusToken='';if(command==='focus-menubar'){focusToken=crypto.randomUUID();focusTargets.clear();focusTargets.set(focusToken,document.activeElement);}return ready.then(()=>request('host-command',{command,focusToken,payload})).then(result=>{if(command==='focus-menubar'&&!result?.focused)focusTargets.delete(focusToken);return result;},error=>{if(focusToken)focusTargets.delete(focusToken);throw error;});},
  shared:{
   get(){return {...shared};},
   set(key,value){return ready.then(()=>request('shared-set',{key,value:value===undefined?null:value})).then(result=>{if(value===null||value===undefined)delete shared[key];else shared[key]=value;return result;});}
  }
 };
 function request(method,data){return new Promise((resolve,reject)=>{const id=++sequence;const timer=setTimeout(()=>{pending.delete(id);reject(Error('The extension host did not respond.'));},120000);pending.set(id,{resolve,reject,timer});port.postMessage({version:1,id,method,...data});});}
 function usable(target){return target?.isConnected&&!target.matches(':disabled')&&!target.closest('[inert],[hidden]')&&target.getClientRects().length&&getComputedStyle(target).visibility==='visible';}
 function restoreChildFocus(saved){const candidates=[saved,...document.querySelectorAll('[data-extension-focus],main'),document.body];for(const target of candidates){if(!target||!usable(target))continue;if(target.tabIndex<0&&!target.hasAttribute('tabindex'))target.tabIndex=-1;target.focus({preventScroll:true});if(document.activeElement===target)return;}}
 function notify(name,data){for(const fn of listeners.get(name)||[])fn(data);}
 function appearance(value){api.theme=value;document.documentElement.dataset.theme=value.mode;document.documentElement.dir=value.direction||'ltr';for(const [key,val]of Object.entries(value.tokens||{})){if(/^--[a-z0-9-]+$/.test(key)&&typeof val==='string'&&val.length<512)document.documentElement.style.setProperty(key,val);}notify('appearance',value);}
 window.addEventListener('message',event=>{
  if(event.source!==parent||event.data?.type!=='dinah-extension-init'||event.data.version!==1||port||event.ports.length!==1)return;
  port=event.ports[0];port.onmessage=event=>{const value=event.data;if(value.version!==1)return;if(value.reply){const p=pending.get(value.reply);if(!p)return;pending.delete(value.reply);clearTimeout(p.timer);value.error?p.reject(Error(value.error)):p.resolve(value.result);return;}if(value.event==='focus-prepare'){const token=value.data?.token;if(typeof token==='string'&&token.length>=16&&token.length<=128){focusTargets.clear();focusTargets.set(token,lastFocused);}}else if(value.event==='focus-return'){const token=value.data?.token;if(!focusTargets.has(token))return;const target=focusTargets.get(token);focusTargets.delete(token);restoreChildFocus(target);}else if(value.event==='appearance')appearance(value.data);else if(value.event==='workspace'){api.workspace=value.data;notify('workspace',value.data);}else if(value.event==='shared'){const change=value.data||{};if(typeof change.key==='string'){if(change.value===null||change.value===undefined)delete shared[change.key];else shared[change.key]=change.value;}notify('shared',change);}else notify(value.event,value.data);};port.start();appearance(event.data.appearance);api.workspace=event.data.workspace;shared=event.data.shared&&typeof event.data.shared==='object'?{...event.data.shared}:{};readyResolve({workspace:api.workspace,state:event.data.state||''});port.postMessage({version:1,method:'ready'});
 },false);
 window.addEventListener('focus',()=>{if(port)port.postMessage({version:1,method:'focus'});});
 document.addEventListener('focusin',event=>{lastFocused=event.target;if(port)port.postMessage({version:1,method:'focus'});});
 document.addEventListener('keydown',event=>{if(event.key==='F10'&&!event.shiftKey&&!event.altKey&&!event.ctrlKey&&!event.metaKey&&!event.isComposing&&!event.repeat&&!event.defaultPrevented){event.preventDefault();api.hostCommand('focus-menubar').catch(()=>{});}});
 window.addEventListener('pagehide',()=>{for(const p of pending.values()){clearTimeout(p.timer);p.reject(Error('Extension frame closed.'));}pending.clear();port?.close();});
})();
