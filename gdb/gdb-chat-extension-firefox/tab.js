// Wait for rbac and GDB to be on window (or directly use imported vars)
// For simplicity, assuming they are loaded as the script tag is type="module" and imports are processed first.

// --- CONFIGURATION ---
// IMPORTANT: For a user to be superadmin on first registration,
// their generated ETH address MUST be in this list.
// You can leave it empty and assign roles later if you build an admin UI.
var currentUserAddress = null;

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

const messagesContainer = document.getElementById('messagesContainer');
const inputMessage = document.getElementById('inputMessage');
const btnSendMessage = document.getElementById('btnSendMessage');
const btnLogout = document.getElementById('btnLogout');

const imgFavicon = document.getElementById('imgFavicon');
const aPageLink = document.getElementById('aPageLink');
const divHash = document.getElementById('divHash');


// --- UI UPDATE LOGIC ---
async function updateUI(securityState) {
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
    try {
        volatileIdentity = await rbac.startNewUserRegistration();
        if (volatileIdentity) {
            newEthAddressElem.textContent = volatileIdentity.address;
            newMnemonicElem.textContent = volatileIdentity.mnemonic;
            // UI update will be handled by securityStateChangeCallback
        } else {
            alert("Failed to generate new identity.");
        }
    } catch (error) {
        console.error("Registration error:", error);
        alert(`Registration error: ${error.message}`);
    }
};

btnProtectWebAuthn.onclick = async () => {
    if (!volatileIdentity || !volatileIdentity.privateKey) {
        alert("No volatile identity (private key) available to protect. Please generate one first.");
        return;
    }
    try {
        const protectedAddress = await rbac.protectCurrentIdentityWithWebAuthn(volatileIdentity.privateKey);
        if (protectedAddress) {
            alert(`Identity ${protectedAddress} protected with WebAuthn and you are now logged in!`);
            volatileIdentity = null; // Clear volatile identity once protected
            // UI update via callback
        } else {
            alert("WebAuthn protection failed. Ensure your browser supports it, you are on HTTPS/localhost, and you completed the WebAuthn prompt.");
        }
    } catch (error) {
        console.error("WebAuthn protection error:", error);
        alert(`WebAuthn protection error: ${error.message}`);
    }
};

btnLoginWebAuthn.onclick = async () => {
    try {
        const loggedInAddress = await rbac.loginCurrentUserWithWebAuthn();
        if (loggedInAddress) {
            alert(`Logged in with WebAuthn as ${loggedInAddress}`);
            await ensureUserRole(loggedInAddress); // Ensure they have 'user' role
        } else {
            alert("WebAuthn login failed. Have you registered WebAuthn for this site?");
        }
    } catch (error) {
        console.error("WebAuthn login error:", error);
        alert(`WebAuthn login error: ${error.message}`);
    }
};

btnLoginMnemonic.onclick = async () => {
    const mnemonic = inputMnemonic.value.trim();
    if (!mnemonic) {
        alert("Please enter your mnemonic phrase.");
        return;
    }
    try {
        const identity = await rbac.loginOrRecoverUserWithMnemonic(mnemonic);
        if (identity) {
            alert(`Logged in with mnemonic for address ${identity.address}`);
            await ensureUserRole(identity.address); // Ensure they have 'user' role
            inputMnemonic.value = ''; // Clear after use
            // User might want to protect this session with WebAuthn now
            // For simplicity, we don't auto-prompt that here.
        } else {
            alert("Failed to login with mnemonic. Please check the phrase.");
        }
    } catch (error) {
        console.error("Mnemonic login error:", error);
        alert(`Mnemonic login error: ${error.message}`);
    }
};

async function ensureUserRole(address) {
    try {
        // We use db.map() to find a node that matches the query
        const {
            results
        } = await db.map({
            query: {
                type: 'role',
                ethAddress: address
            },
            $limit: 1 // We only need to know if at least one exists
        });

        // If the results array is not empty, the role already exists
        if (results.length > 0) {
            console.log(`User ${address} already has a role.`);
            return;
        }

        // If not, we assign the 'user' role
        console.log(`Assigning 'user' role to ${address}...`);
        await rbac.assignRole(address, 'user');
        console.log(`Role 'user' assigned to ${address}`);
    } catch (error) {
        console.error("Failed during role check/assignment:", error);
    }
}

btnLogout.onclick = async () => {
    try {
        await rbac.clearSecurity();
        alert("You have been logged out.");
    } catch (error) {
        console.error("Logout error:", error);
        alert(`Logout error: ${error.message}`);
    }
};

// --- CHAT FUNCTIONALITY ---
function displayMessage({
    id,
    value,
    action
}) {
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

    msgElement.innerHTML = `
                <span class="sender">${value.sender === currentUserAddress ? 'You' : senderShort}</span>
                <span class="text">${value.text}</span>
                <span class="timestamp">${messageDate}</span>
            `;

    // Scroll to bottom for new messages
    if (action === 'added' || action === 'initial') {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
}

btnSendMessage.onclick = async () => {
    const text = inputMessage.value.trim();
    if (!text) return;

    try {
        // Check permission to send message (defined as 'write' in custom roles)
        const senderAddress = await rbac.executeWithPermission('write');

        const messageData = {
            type: 'message',
            hash: pageHash,
            sender: senderAddress,
            text: text,
            timestamp: Date.now()
        };
        await db.put(messageData);
        inputMessage.value = ''; // Clear input field
        // Message will appear via the real-time 'map' listener
    } catch (error) {
        console.error("Failed to send message:", error);
        alert(`Failed to send message: ${error.message}. Do you have 'write' permission?`);
    }
};



//// MMMM
let pageHash = "";
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
        chrome.runtime.sendMessage(messageClone);
    } else if (message.action === "showAlert") {
        alert(message.message); // Simple, replace with custom modal/toast if needed.
    }
    else if (message.action === "updateUI")
    {
      updateUI(message.securityState);
    }
    else if (message.action === "displayMessage" && message.myData.value.hash === pageHash)
    {
      displayMessage(message.myData.id, message.myData.value, message.myData.action);
    }
    else if (message.action === "statusBarUISet")
    {
      statusBar.textContent = message.text;
    }
    else if (message.action ==="clearMessagesContainer" && message.hash === pageHash)
    {
      messagesContainer.innerHTML = ''; // Clear previous messages
    }
});