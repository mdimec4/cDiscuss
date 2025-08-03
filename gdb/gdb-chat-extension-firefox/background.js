let securityContextActive = false;
let activeMnemnonic = "";

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.action === 'checkSecurityContext') {
        sendResponse({ isActive: securityContextActive });
    } else if (message.action === 'setSecurityContextActive') {
        securityContextActive = message.value;
	} else if (message.action === 'getMnemonic') {
		sendResponse({ activeMnemnonic: activeMnemnonic });
	} else if (message.action === 'setMnemonic') {
		activeMnemnonic = message.value;
	}
});