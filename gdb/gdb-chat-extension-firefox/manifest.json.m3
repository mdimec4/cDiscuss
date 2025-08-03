{
  "manifest_version": 3,
  "name": "GDB Chat Extension",
  "version": "1.0",
  "description": "Page-specific chat using GenosDB",
  "permissions": ["storage", "activeTab", "scripting"],
  "action": {
    "default_popup": "popup.html"
  },
 ///// "content_security_policy": {
    "extension_pages": "script-src 'self'; object-src 'self'; worker-src 'self' blob:"
  }////
  "background": {
    "service_worker": "background.js"
  }
}