// 与 src/browser/dom-helpers.ts waitForDomStableJs(2000, 500) 一致，供 Navigate 后等待 SPA 稳定。
new Promise(resolve => {
  if (!document.body) {
    setTimeout(() => resolve('nobody'), 2000);
    return;
  }
  let timer = null;
  let cap = null;
  const done = (reason) => {
    clearTimeout(timer);
    clearTimeout(cap);
    obs.disconnect();
    resolve(reason);
  };
  const resetQuiet = () => {
    clearTimeout(timer);
    timer = setTimeout(() => done('quiet'), 500);
  };
  const obs = new MutationObserver(resetQuiet);
  obs.observe(document.body, { childList: true, subtree: true, attributes: true });
  resetQuiet();
  cap = setTimeout(() => done('capped'), 2000);
})
