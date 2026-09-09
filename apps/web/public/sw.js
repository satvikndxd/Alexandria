/* Alexandria service worker: push display only.
   No caching, no tracking, no fetch interception — the worker exists so the
   platform can show a notification the reader opted into, and for nothing
   else. */
self.addEventListener("push", (event) => {
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch (e) {
    data = { body: String(event.data || "") };
  }
  event.waitUntil(
    self.registration.showNotification(data.title || "Alexandria", {
      body: data.body || "",
      tag: "alexandria-notification",
      data: { url: data.url || "/notifications" },
    })
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = (event.notification.data && event.notification.data.url) || "/notifications";
  event.waitUntil(
    self.clients.matchAll({ type: "window" }).then((list) => {
      for (const client of list) {
        if ("focus" in client) return client.focus();
      }
      return self.clients.openWindow(url);
    })
  );
});
