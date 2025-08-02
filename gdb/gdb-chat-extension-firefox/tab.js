// Wait for rbac and GDB to be on window (or directly use imported vars)
        // For simplicity, assuming they are loaded as the script tag is type="module" and imports are processed first.

        import { GDB } from "/vendor/gdb.min.js"; // Adjust path according to your structure
        import * as rbac from '/vendor/rbac.min.js'; // Adjust path according to your structure

        // --- CONFIGURATION ---
        // IMPORTANT: For a user to be superadmin on first registration,
        // their generated ETH address MUST be in this list.
        // You can leave it empty and assign roles later if you build an admin UI.
        const SUPERADMIN_ADDRESSES = ["0x0707eeB901d6c78bD0b3b31C2C6F5E00DF8f26Dd", "0x6ac5a9DB5539A2595fb5F63EDb7Cf3C601322006"]; // Replace or add your desired superadmin ETH address(es)

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
		const divHash  = document.getElementById('divHash');

        // --- APP STATE ---
        let db;
        let volatileIdentity = null; // To store { address, mnemonic, privateKey } temporarily
        let unsubscribeMessages = null;
        let currentUserAddress = null;

        // --- RBAC Custom Roles ---
        const CHAT_APP_ROLES = {
          superadmin: { can: ["assignRole", "deleteAnyMessage", "write"], inherits: ["admin"] }, // Example
          admin: { can: ["deleteMessage"], inherits: ["user"] }, // Example
          user: { can: ["write", "readSelf"], inherits: ["guest"] }, // "write" allows sending messages
          guest: { can: ["read", "write", "sync"] }, // Default, can read public messages
        };

        // --- UI UPDATE LOGIC ---
        function updateUI(securityState) {
            if (!securityState) {
                statusBar.textContent = "Status: Security context not active.";
                authSection.classList.remove('hidden');
                chatSection.classList.add('hidden');
                currentUserAddress = null;
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
                loadMessages();
            } else {
                authSection.classList.remove('hidden');
                chatSection.classList.add('hidden');
                if (unsubscribeMessages) {
                    unsubscribeMessages();
                    unsubscribeMessages = null;
                }
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
        
        // --- INITIALIZATION ---
        async function initializeApp() {
            try {
                statusBar.textContent = "Status: Initializing DB...";
                db = new GDB("cDiscuss-DB");

                statusBar.textContent = "Status: DB Ready. Initializing Security Context...";

                await rbac.createSecurityContext(db, SUPERADMIN_ADDRESSES);

                rbac.setCustomRoles(CHAT_APP_ROLES);
                rbac.setSecurityStateChangeCallback(updateUI);
                
                // Trigger initial UI update based on current state (e.g. from silent WebAuthn login)
                // The callback itself will be called by createSecurityContext, but to be safe:
                const initialState = {
                    isActive: rbac.isSecurityActive(),
                    activeAddress: rbac.getActiveEthAddress(),
                    isWebAuthnProtected: rbac.isCurrentSessionProtectedByWebAuthn(),
                    hasVolatileIdentity: !!rbac.getMnemonicForDisplayAfterRegistrationOrRecovery(), // Heuristic
                    hasWebAuthnHardwareRegistration: rbac.hasExistingWebAuthnRegistration()
                };
                
                updateUI(initialState);
                
                // Attempt silent WebAuthn login if available
                if (rbac.hasExistingWebAuthnRegistration() && !rbac.isSecurityActive()) {
                    console.log("Attempting silent WebAuthn login...");
                    await rbac.loginCurrentUserWithWebAuthn().catch(err => console.warn("Silent WebAuthn login failed or no registration:", err.message));
                }


            } catch (error) {
                console.error("Initialization failed:", error);
                statusBar.textContent = `Error: ${error.message}`;
                alert(`Initialization Error: ${error.message}`);
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
        function displayMessage({ id, value, action }) {
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

        async function loadMessages() {
            if (unsubscribeMessages) {
                unsubscribeMessages(); // Unsubscribe from previous listener if any
            }
            messagesContainer.innerHTML = ''; // Clear previous messages

            try {
                const { unsubscribe } = await db.map(
                    { query: { type: 'message', hash: pageHash }, field: 'timestamp', order: 'asc' },
                    displayMessage // ({ id, value, action, edges, timestamp })
                );
                unsubscribeMessages = unsubscribe;
            } catch (error) {
                console.error("Failed to load messages:", error);
                alert(`Failed to load messages: ${error.message}`);
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
        
        //// Start the application
        //initializeApp();
		
		
//// MMMM
let pageHash = "";
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    console.log("Received in tab:", message.myData);
	pageHash = message.myData.urlHash;
	
	imgFavicon.src = message.myData.faviconUrl;
	aPageLink.href = message.myData.pageUrl;
	aPageLink.textContent = message.myData.title;
	document.title = "Chat: " + message.myData.title;
	divHash.textContent = message.myData.urlHash;
	// Start the application
    initializeApp();
});