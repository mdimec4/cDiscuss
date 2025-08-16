// MMMMMMMMMMMMMMMM
function sha512(str) {
    return crypto.subtle.digest("SHA-512", new TextEncoder("utf-8").encode(str)).then(buf => {
        return Array.prototype.map.call(new Uint8Array(buf), x=>(('00'+x.toString(16)).slice(-2))).join('');
        });
}
       

chrome.tabs.query({ active: true, currentWindow: true }, function (tabs) {

            
                if (tabs.length > 0) {
                    const tab = tabs[0];
                    console.log("URL:", tab.url);
                    console.log("Title:", tab.title);
                    console.log("Favicon URL:", tab.favIconUrl);
                    sha512(tab.url).then(function (hash) {
                          console.log("Page hash:", hash);
                          // Start the application tab
                          chrome.tabs.create({ url: chrome.runtime.getURL("tab.html") }, function(newTab) {
                            // 2. Send data to the new tab after it loads
                             chrome.tabs.onUpdated.addListener(function listener(tabId, changeInfo) {
                                if (tabId === newTab.id && changeInfo.status === 'complete') {
                                    // Send message directly to content script in new tab
                                    chrome.runtime.sendMessage({ tabId: tab.id, action "popupInit", myData: { pageUrl: tab.url, urlHash: hash, title: tab.title, faviconUrl:  tab.favIconUrl, newExtensionTabId: newTab.id} });
                                    chrome.tabs.onUpdated.removeListener(listener);
                                }
                             });
                          });
                    });            
                }
});


        

