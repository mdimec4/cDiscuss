// Wait for rbac and GDB to be on window (or directly use imported vars)
// For simplicity, assuming they are loaded as the script tag is type="module" and imports are processed first.

// --- CONFIGURATION ---
// IMPORTANT: For a user to be superadmin on first registration,
// their generated ETH address MUST be in this list.
// You can leave it empty and assign roles later if you build an admin UI.
var currentUserAddress = null;
let pageHash = "";

// --- DOM Elements ---
const authSection = document.getElementById('authSection');
const chatSection = document.getElementById('chatSection');
const statusBar = document.getElementById('statusBar');

const btnRegisterNew = document.getElementById('btnRegisterNew');
const newIdentityInfo = document.getElementById('newIdentityInfo');
const newEthAddressElem = document.getElementById('newEthAddress');
const newMnemonicElem = document.getElementById('newMnemonic');
const btnProtectWebAuthn = document.getElementById('btnProtectWebAuthn');

const btnLoginWebAuthn = document.getElementById('btnLoginWebAuthn');
const inputMnemonic = document.getElementById('inputMnemonic');
const btnLoginMnemonic = document.getElementById('btnLoginMnemonic');
const btnRegisterWebAuthn = document.getElementById('btnRegisterWebAuthn');

const messagesContainer = document.getElementById('messagesContainer');
const inputMessage = document.getElementById('inputMessage');
const btnSendMessage = document.getElementById('btnSendMessage');
const btnLogout = document.getElementById('btnLogout');

const imgFavicon = document.getElementById('imgFavicon');
const aPageLink = document.getElementById('aPageLink');
const divHash = document.getElementById('divHash');


// --- UI UPDATE LOGIC ---
function updateUI(securityState, currentUserRole) {
    if (!securityState) {
        statusBar.textContent = "Status: Security context not active.";
        authSection.classList.remove('hidden');
        chatSection.classList.add('hidden');
        return;
    }

    currentUserAddress = securityState.activeAddress;
    let statusText = `Status: ${securityState.isActive ? `Logged in as ${securityState.activeAddress.substring(0,10)}...` : 'Logged out.'}`;
    statusText += ` | WebAuthn Active: ${securityState.isWebAuthnProtected}`;
    statusText += ` | WebAuthn Registered Here: ${securityState.hasWebAuthnHardwareRegistration}`;
    if (currentUserRole) statusText += ` | Role: ${currentUserRole}`;
    statusBar.textContent = statusText;

    btnLoginWebAuthn.disabled = !securityState.hasWebAuthnHardwareRegistration;

    if (securityState.isActive) {
        authSection.classList.add('hidden');
        chatSection.classList.remove('hidden');
        newIdentityInfo.classList.add('hidden'); // Hide registration info if logged in
    } else {
        authSection.classList.remove('hidden');
        chatSection.classList.add('hidden');
        messagesContainer.innerHTML = ''; // Clear messages on logout
    }

    if (securityState.hasVolatileIdentity) {
        newIdentityInfo.classList.remove('hidden');
        btnRegisterNew.disabled = true;
    } else {
        newIdentityInfo.classList.add('hidden');
        btnRegisterNew.disabled = false;
    }
}


// --- IDENTITY MANAGEMENT HANDLERS ---
btnRegisterNew.onclick = async () => {
    chrome.runtime.sendMessage({
        action: "registerNew"
    }, (response) => {
        if (!response) return;
        if (response.error) {
            alert(response.error);
            return;
        }
        newEthAddressElem.textContent = response.address;
        newMnemonicElem.textContent = response.mnemonic;
    });
};

// Copy mnemonic to clipboard and reflect short visual feedback
const btnCopyMnemonic = document.getElementById('btnCopyMnemonic');
btnCopyMnemonic.onclick = () => {
    // Paste mnemonic into the login field and trigger verification via input event
    const mnemonic = newMnemonicElem.textContent;
    navigator.clipboard.writeText(mnemonic).then(() => {
        const originalText = btnCopyMnemonic.textContent;
        btnCopyMnemonic.textContent = 'Copied!';
        setTimeout(() => btnCopyMnemonic.textContent = originalText, 1500);
    });
    inputMnemonic.value = mnemonic;
    // Dispara la verificación como si el usuario hubiera pegado manualmente
    inputMnemonic.dispatchEvent(new Event('input'));
};

btnProtectWebAuthn.onclick = async () => {
    chrome.runtime.sendMessage({
        action: "protectWebAuthn"
    }, (response) => {
        if (!response) retuern;
        if (response.message)
            alert(response.message);
        else if (response.error)
            alert(response.error);
    });
};

btnLoginWebAuthn.onclick = async () => {
    chrome.runtime.sendMessage({
        action: "loginWebAuthn"
    }, (response) => {
        if (!response) retuern;
        if (response.message)
            alert(response.message);
        if (response.error)
            alert(response.error);
    });
};

//  !! TODO implement in background !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
// On paste/type mnemonic, do NOT login automatically. Validate shape and suggest WebAuthn.
inputMnemonic.addEventListener('input', async () => {
    const mnemonic = inputMnemonic.value.trim();
    // Simple heuristic validation: 12-24 words
    const words = mnemonic.split(/\s+/).filter(Boolean);
    const looksValid = words.length >= 12; // keep permissive; actual check occurs on login
    if (!looksValid) {
        btnLoginMnemonic.disabled = true;
        btnRegisterWebAuthn.style.display = 'none';
        return;
    }
    // Enable login but do not start session automatically
    btnLoginMnemonic.disabled = false;
    try {
        const hasReg = sm.hasExistingWebAuthnRegistration();
        // If device has no WebAuthn registration, offer to register
        btnRegisterWebAuthn.style.display = hasReg ? 'none' : '';
        btnRegisterWebAuthn.disabled = !!hasReg;
        // Click handler will be bound after actual login (when we have privateKey),
        // or we can prompt the user to login first if needed.
        btnRegisterWebAuthn.onclick = async () => {
            // To register, we need the private key; prompt a quick, temporary login then protect.
            try {
                const identity = await sm.loginOrRecoverUserWithMnemonic(mnemonic);
                if (!identity) return;
                const addr = await sm.protectCurrentIdentityWithWebAuthn(identity.privateKey);
                if (!addr) return; // Registration failed (error already alerted by SWM)
                alert('WebAuthn registered successfully.');
                btnRegisterWebAuthn.style.display = 'none';
            } catch (error) {
                alert('Error registering WebAuthn: ' + error.message);
            }
        };
    } catch (err) {
        console.warn('Unable to check WebAuthn registration:', err?.message || err);
        btnRegisterWebAuthn.style.display = 'none';
    }
});
// TODO !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

btnLoginMnemonic.onclick = async () => {
    const mnemonic = inputMnemonic.value.trim();
    if (!mnemonic) {
        alert("Please enter your mnemonic phrase.");
        return;
    }

    chrome.runtime.sendMessage({
        action: "loginMnemonic",
        mnemonic: mnemonic
    }, (response) => {
        if (!response) return;
        if (response.message)
            alert(response.message);
        if (response.error)
            alert(response.error);
        if (response.mnemonic)
            inputMnemonic.value = response.mnemonic;
    });
};

btnLogout.onclick = async () => {
    chrome.runtime.sendMessage({
        action: "logout"
    }, (response) => {
        if (!response) retuern;
        if (response.message)
            alert(response.message);
        if (response.error)
            alert(response.error);
    });
};

// --- CHAT FUNCTIONALITY ---
function displayMessage({
        id,
        value,
        action
    },
    currentUserRole,
    isSuperadminAddress) {
    if (action === 'removed') {
        const msgElement = document.getElementById(`msg-${id}`);
        if (msgElement) msgElement.remove();
        return;
    }
    if (!value || value.type !== 'message') return; // Only process message type nodes

    let msgElement = document.getElementById(`msg-${id}`);
    if (action === 'updated' && !msgElement) return; // Should not happen if initial/added handled

    if (!msgElement) { // 'initial' or 'added'
        msgElement = document.createElement('div');
        msgElement.id = `msg-${id}`;
        msgElement.classList.add('message');
        messagesContainer.appendChild(msgElement);
    }

    msgElement.classList.toggle('own', value.sender === currentUserAddress);
    msgElement.classList.toggle('other', value.sender !== currentUserAddress);

    const senderShort = value.sender ? `${value.sender.substring(0, 6)}...${value.sender.substring(value.sender.length - 4)}` : 'Unknown';
    const messageDate = value.timestamp ? new Date(value.timestamp).toLocaleString() : 'No timestamp';
    const role = value.role || (value.sender === currentUserAddress ? currentUserRole : '');
    const canDelete = (value.sender === currentUserAddress) || ['admin', 'superadmin'].includes(currentUserRole) ||
        isSuperadminAddress;

    msgElement.innerHTML = `
                <span class="sender">${value.sender === currentUserAddress ? 'You' : senderShort}<span class="role">${role ? ' (' + role + ')' : ''}</span></span>
                <span class="text">${value.text}</span>
                <span class="timestamp">${messageDate}</span>
                ${canDelete ? `<button class="delete-btn" title="Delete" onclick="window.deleteMessage && window.deleteMessage('${id}')">🗑️</button>` : ''}
            `;

    // Scroll to bottom for new messages
    if (action === 'added' || action === 'initial') {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
}

/**
 * Deletes a message if permitted by current user's role.
 * @param {string} id Node ID to delete
 */
window.deleteMessage = async (id) => { // TODO Implement in background !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
    if (!confirm("Delete this message?")) return;
    try {
        const active = sm.getActiveEthAddress();
        if (!active) {
            alert('Login required.');
            return;
        }
        if (!isSuperadminAddress(active)) {
            await sm.executeWithPermission('delete');
        }
        await db.remove(id);
    } catch (error) {
        alert("You do not have permission to delete this message.");
    }
}; // TODO !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

btnSendMessage.onclick = async () => {
    const text = inputMessage.value.trim(); // TODO
    if (!text) return;

    chrome.runtime.sendMessage({
        action: "sendMessage",
        hash: pageHash,
        text: text
    }, (response) => {
        if (!response) return;
        if (response.error) {
            alert(response.error);
            return;
        }
        inputMessage.value = response.text;
    });
};

// Send message on Enter (without Shift)
inputMessage.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        btnSendMessage.click();
    }
});



chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.action === "popupInit") {
        if (pageHash !== "")
            return;

        console.log("Received in tab:", message.myData);
        pageHash = message.myData.urlHash;

        imgFavicon.src = message.myData.faviconUrl;
        aPageLink.href = message.myData.pageUrl;
        aPageLink.textContent = message.myData.title;
        document.title = "Chat: " + message.myData.title;
        divHash.textContent = message.myData.urlHash;

        // inform background
        const messageClone = structuredClone(message);
        messageClone.action = "extensionTabInit";
        chrome.runtime.sendMessage(messageClone, (response) => {
            if (response && response.error) {
                alert(response.error);
            }
        });
    } else if (message.action === "showAlert") {
        alert(message.message); // Simple, replace with custom modal/toast if needed.
    } else if (message.action === "updateUI") {
        updateUI(message.securityState, message.currentUserRole);
    } else if (message.action === "displayMessage" && message.myData.value.hash === pageHash) {
        displayMessage(message.myData, message.currentUserRole, message.isSuperadminAddress);
    } else if (message.action === "statusBarUISet") {
        statusBar.textContent = message.text;
    } else if (message.action === "clearMessagesContainer" && message.hash === pageHash) {
        messagesContainer.innerHTML = ''; // Clear previous messages
    }
});