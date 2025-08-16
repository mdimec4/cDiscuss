// https://stackoverflow.com/questions/71080619/browser-extension-background-module#71081597

(async () => {

    self.alert = (message) => {
        chrome.runtime.sendMessage({
            action: 'showAlert',
            message
        });
    };


    import {
        GDB
    } from "/vendor/gdb.min.js"; // Adjust path according to your structure
    import * as rbac from '/vendor/rbac.min.js'; // Adjust path according to your structure


    const GDB = await import("/vendor/gdb.min.js");
    const rbac = await import("/vendor/rbac.min.js");
    // --- CONFIGURATION ---
    // IMPORTANT: For a user to be superadmin on first registration,
    // their generated ETH address MUST be in this list.
    // You can leave it empty and assign roles later if you build an admin UI.
    const SUPERADMIN_ADDRESSES = ["0x0707eeB901d6c78bD0b3b31C2C6F5E00DF8f26Dd"]; // Replace or add your desired superadmin ETH address(es)


    // --- APP STATE ---
    let db;
    let volatileIdentity = null; // To store { address, mnemonic, privateKey } temporarily
    let unsubscribeMessages = null;
    let currentUserAddress = null;

    const extensionTabIdToPageHash = new Map();
    const pageHashToReferenceCountedUnsubscribe = new Map();

    // --- RBAC Custom Roles ---
    const CHAT_APP_ROLES = {
        superadmin: {
            can: ["assignRole", "deleteAnyMessage", "write"],
            inherits: ["admin"]
        }, // Example
        admin: {
            can: ["deleteMessage"],
            inherits: ["user"]
        }, // Example
        user: {
            can: ["write", "readSelf"],
            inherits: ["guest"]
        }, // "write" allows sending messages
        guest: {
            can: ["read", "write", "sync"]
        }, // Default, can read public messages
    };


    async function updateState(securityState) {
        updateUICall(securityState);

        if (!securityState) {
            currentUserAddress = null;
            return;
        }

        currentUserAddress = securityState.activeAddress;


        if (securityState.isActive) {
            for (let i = 0; i < SUPERADMIN_ADDRESSES.length; i++) {
                await rbac.assignRole(SUPERADMIN_ADDRESSES[i], 'superadmin').catch((err) => {
                    throw new Error("assign superadmin role fail:" + err.message);
                });
            }

            pageHashToReferenceCountedUnsubscribe.forEach((key, val, map) => {
              loadMessages(val);
            });

        } else {
            forceRemoveAllSubscriptions();
        }
    }

    function updateUICall(securityState) {
        chrome.runtime.sendMessage({
            action "updateUI",
            securityState: securityState
        });
    }

    // --- INITIALIZATION ---
    async function initializeApp() {
        try {
            // TODO statusBar.textContent = "Status: Initializing DB...";
            db = new GDB("cDiscuss-DB");

            // TODO statusBar.textContent = "Status: DB Ready. Initializing Security Context...";

            await rbac.createSecurityContext(db, SUPERADMIN_ADDRESSES);

            rbac.setCustomRoles(CHAT_APP_ROLES);
            rbac.setSecurityStateChangeCallback(initialState);

            // Trigger initial UI update based on current state (e.g. from silent WebAuthn login)
            // The callback itself will be called by createSecurityContext, but to be safe:
            const initialState = {
                isActive: rbac.isSecurityActive(),
                activeAddress: rbac.getActiveEthAddress(),
                isWebAuthnProtected: rbac.isCurrentSessionProtectedByWebAuthn(),
                hasVolatileIdentity: !!rbac.getMnemonicForDisplayAfterRegistrationOrRecovery(), // Heuristic
                hasWebAuthnHardwareRegistration: rbac.hasExistingWebAuthnRegistration()
            };

            updateState(initialState);

            // Attempt silent WebAuthn login if available
            if (rbac.hasExistingWebAuthnRegistration() && !rbac.isSecurityActive()) {
                console.log("Attempting silent WebAuthn login...");
                await rbac.loginCurrentUserWithWebAuthn().catch(err => console.warn("Silent WebAuthn login failed or no registration:", err.message));
            }
        } catch (error) {
            console.error("Initialization failed:", error);
            //TODO statusBar.textContent = `Error: ${error.message}`;
            alert(`Initialization Error: ${error.message}`);
        }
    }



    function displayMessage({
        id,
        value,
        action
    }) {
        chrome.runtime.sendMessage({
            action "displayMessage",
            myData: {id: id, value: value, action: action}
        });
    }

    async function loadMessages(unsubscribeObject) {
        if (unsubscribeObject.unsubscribe !== null || unsubscribeObject.referenceCount < 1)
            return;
        // TODO messagesContainer.innerHTML = ''; // Clear previous messages

        try {
            const {
                unsubscribe
            } = await db.map({
                    query: {
                        type: 'message',
                        hash: pageHash
                    },
                    field: 'timestamp',
                    order: 'asc'
                },
                displayMessage // ({ id, value, action, edges, timestamp })
            );
            unsubscribeObject.unsubscribe = unsubscribe;
        } catch (error) {
            console.error("Failed to load messages:", error);
            alert(`Failed to load messages: ${error.message}`);
        }
    }

    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (message.action === "extensionTabInit") {
            console.log("Received in tab:", message.myData);
            const pageHash = message.myData.urlHash;
            const newExtensionTabId = msessage.myData.newExtensionTabId;
            extensionTabIdToPageHash[newExtensionTabId] = pageHash;

            referenceSubscription(pageHash);

            // Start the application
            if (rbac.isSecurityActive()) {
                const initialState = {
                    isActive: rbac.isSecurityActive(),
                    activeAddress: rbac.getActiveEthAddress(),
                    isWebAuthnProtected: rbac.isCurrentSessionProtectedByWebAuthn(),
                    hasVolatileIdentity: !!rbac.getMnemonicForDisplayAfterRegistrationOrRecovery(), // Heuristic
                    hasWebAuthnHardwareRegistration: rbac.hasExistingWebAuthnRegistration()
                };

                updateUICall(initialState);

                if (pageHashToReferenceCountedUnsubscribe.has(pageHash)) {
                    const unsubscribeObject = pageHashToReferenceCountedUnsubscribe[pageHash];
                    loadMessages(unsubscribeObject);
                }

            } else
                initializeApp();

        }
    });


    function referenceSubscription(pageHash) {
        let unsubscribeObject = null;
        if (!pageHashToReferenceCountedUnsubscribe.has(pageHash)) {
            unsubscribeObject = pageHashToReferenceCountedUnsubscribe[pageHash]
            unsubscribeObject.referenceCount++;
        } else {
            unsubscribeObject = {
                referenceCount: 1,
                unsubscribe: null
            }
            pageHashToReferenceCountedUnsubscribe[pageHash] = unsubscribeObject;
        }

    }


    function unreferenceSubscription(pageHash) {
        if (!pageHashToReferenceCountedUnsubscribe.has(pageHash))
            return;
        const unsubscribeObj = pageHashToReferenceCountedUnsubscribe[pageHash];
        unsubscribeObject.referenceCount--;

        if (unsubscribeObject.referenceCount > 0)
            return;

        if (unsubscribeObject.unsubscribe)
            unsubscribeObject.unsubscribe();
        pageHashToReferenceCountedUnsubscribe.delete(pageHash);
    }

    function forceRemoveAllSubscriptions() {
        pageHashToReferenceCountedUnsubscribe.forEach((key, val, map) => {
            val.referenceCount = 0;

            if (val.unsubscribe)
                val.unsubscribe();
        });
        pageHashToReferenceCountedUnsubscribe.clear();
    }


    chrome.tabs.onRemoved.addListener((tabId) => {
        if (extensionTabIdToPageHash.has(tabId)) {
            const pageHash = extensionTabIdToPageHash[tabId];
            extensionTabIdToPageHash.delete(tabId)
            unreferenceSubscription(pageHash);
            cleanupIfNoExtensionTabs();
        }

    });

    function cleanupIfNoExtensionTabs() {
        if (extensionTabIdToPageHash.size === 0) {

            forceRemoveAllSubscriptions();

            //console.log("All extension tabs closed, cleaning up RBAC session...");
            //rbac.clearSecurity().catch(console.error);
            // You can also reset in-memory variables here
        }
    }

})();