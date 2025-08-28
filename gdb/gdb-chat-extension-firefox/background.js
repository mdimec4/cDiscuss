// https://stackoverflow.com/questions/71080619/browser-extension-background-module#71081597

(async () => {

    self.alert = (message) => {
        chrome.runtime.sendMessage({
            action: 'showAlert',
            message
        });
    };

    const GDB = await import("/vendor/index.js");

    // --- CONFIGURATION ---
    // IMPORTANT: For a user to be superadmin on first registration,
    // their generated ETH address MUST be in this list.
    // You can leave it empty and assign roles later if you build an admin UI.
    const SUPERADMIN_ADDRESSES = ["0x0707eeB901d6c78bD0b3b31C2C6F5E00DF8f26Dd"]; // Replace or add your desired superadmin ETH address(es)


    // --- APP STATE ---
    // Normalize superadmin addresses to be case-insensitive
    const SUPERADMIN_SET = new Set(SUPERADMIN_ADDRESSES.map(a => a.toLowerCase()));
    const isSuperadminAddress = (addr) => !!addr && SUPERADMIN_SET.has(addr.toLowerCase());
    let db;
    let sm;
    let volatileIdentity = null; // To store { address, mnemonic, privateKey } temporarily
    let unsubscribeMessages = null;
    let currentUserAddress = null;
    let currentUserRole = null;

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
        try {
            if (!securityState) {
                currentUserAddress = null;
                currentUserRole = null;
                return;
            }

            currentUserAddress = securityState.activeAddress;
            if (securityState.isActive) {

                if (isSuperadminAddress(currentUserAddress)) {
                    currentUserRole = 'superadmin';
                } else {
                    try {
                        await ensureUserRole(currentUserAddress);
                    } catch (error) {
                        console.error(`Ensure user role: ${error.message}`);
                    }
                }

                for (let i = 0; i < SUPERADMIN_ADDRESSES.length; i++) {
                    await sm.assignRole(SUPERADMIN_ADDRESSES[i], 'superadmin').catch((err) => {
                        throw new Error("assign superadmin role fail:" + err.message);
                    });
                }

                pageHashToReferenceCountedUnsubscribe.forEach((val, key, map) => {
                    loadMessages(key, val, false);
                });

            } else {
                currentUserRole = null;
                forceUnsubscribeAll();
            }
        } finally {
            updateUICall(securityState);
        }

    }

    function updateUICall(securityState, currentUserRole) {
        chrome.runtime.sendMessage({
            action: "updateUI",
            securityState: securityState,
            currentUserRole: currentUserRole
        });
    }

    /**
     * Ensures the user has a role; assigns 'user' if none exists.
     * @param {string} address Ethereum address
     * @returns {Promise<void>}
     */
    const ensureUserRole = async address => {
        try {
            // Preferred: user:<address> node, as in the RBAC example app
            const {
                result: userNode
            } = await db.get(`user:${address}`);
            if (userNode?.value?.role) {
                currentUserRole = userNode.value.role;
                return;
            }
            // Fallback: legacy role storage
            const {
                results
            } = await db.map({
                query: {
                    type: 'role',
                    ethAddress: address
                },
                $limit: 1
            });
            currentUserRole = results[0]?.value?.role ?? 'guest';
        } catch (error) {
            console.error("Error ensuring role:", error);
            currentUserRole = 'guest';
        }
    };


    // --- IDENTITY MANAGEMENT HANDLERS ---
    async function registerNew(sendResponse) {
        try {
            volatileIdentity = await sm.startNewUserRegistration();
            if (volatileIdentity) {
                sendResponse({
                    address: volatileIdentity.address,
                    mnemonic: volatileIdentity.mnemonic
                });
                // UI update will be handled by securityStateChangeCallback
            } else {
                sendResponse({
                    error: "Failed to generate new identity."
                });
            }
        } catch (error) {
            sendResponse({
                error: "Failed to generate new identity."
            });
            console.error(`Registration error: ${error.message}`);
        }
    };

    async function protectWebAuthn(sendResponse) {
        if (!volatileIdentity || !volatileIdentity.privateKey) {
            sendResponse({
                error: "No volatile identity (private key) available to protect. Please generate one first."
            });
            return;
        }
        try {
            const protectedAddress = await sm.protectCurrentIdentityWithWebAuthn(volatileIdentity.privateKey);
            if (protectedAddress) {
                sendResponse({
                    message: `Identity ${protectedAddress} protected with WebAuthn and you are now logged in!`
                });
                volatileIdentity = null; // Clear volatile identity once protected
                // UI update via callback
            } else {
                sendResponse({
                    error: "WebAuthn protection failed. Ensure your browser supports it, you are on HTTPS/localhost, and you completed the WebAuthn prompt."
                });
            }
        } catch (error) {
            console.error("WebAuthn protection error:", error);
            sendResponse({
                error: `WebAuthn protection error: ${error.message}`
            });
        }
    };

    async function loginWebAuthn(sendResponse) {
        try {
            const loggedInAddress = await sm.loginCurrentUserWithWebAuthn();
            if (!loggedInAddress) {
                sendResponse({
                    error: "WebAuthn login failed. Have you registered WebAuthn for this site?"
                });
                return;
            }
            if (!isSuperadminAddress(identity.address)) {
                await ensureUserRole(identity.address);
            } else {
                currentUserRole = 'superadmin';
            }
            sendResponse({
                message: `Logged in with WebAuthn as ${loggedInAddress}`
            });

        } catch (error) {
            console.error("WebAuthn login error:", error);
            sendResponse({
                error: `WebAuthn login error: ${error.message}`
            });
        }
    };

    // TODO tu si ostal !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
    // https://github.com/estebanrfp/gdb/blob/main/examples/chatrbac.html
    
    async function loginMnemonic(mnemonic, sendResponse) {
        try {
            const identity = await sm.loginOrRecoverUserWithMnemonic(mnemonic);
            if (identity) {
                // If it's not a superadmin address, ensure it has at least 'user' role
                if (!isSuperadminAddress(identity.address)) {
                    await ensureUserRole(identity.address);
                } else {
                    currentUserRole = 'superadmin';
                }
                sendResponse({
                    mnemonic: "",
                    message: `Logged in with mnemonic for address ${identity.address}`
                }); // Clear after use
                // User might want to protect this session with WebAuthn now
                // For simplicity, we don't auto-prompt that here.
            } else {
                sendResponse({
                    error: "Failed to login with mnemonic. Please check the phrase."
                });
            }
        } catch (error) {
            console.error("Mnemonic login error:", error);
            sendResponse({
                error: `Mnemonic login error: ${error.message}`
            });
        }
    };

    async function logout(sendResponse) {
        try {
            await sm.clearSecurity();
            sendResponse({
                message: "You have been logged out."
            });
        } catch (error) {
            console.error("Logout error:", error);
            sendResponse({
                error: `Logout error: ${error.message}`
            });
        }
    };

    async function sendMessage(pageHash, text, sendResponse) {
        try {
            const senderAddress = sm.getActiveEthAddress();
            if (!sm.isSecurityActive() || !senderAddress) {
                alert('Login required to send messages.');
                return;
            }

            const messageData = {
                type: 'message',
                hash: pageHash,
                sender: senderAddress,
                text: text,
                timestamp: Date.now(),
                role: currentUserRole
            };
            await db.put(messageData);
            sendResponse({
                text: ""
            });
            // Message will appear via the real-time 'map' listener
        } catch (error) {
            console.error("Failed to send message:", error);
            sendResponse({
                error: `Failed to send message: ${error.message}. Do you have 'write' permission?`
            });
        }
    };

    // --- INITIALIZATION ---
    async function initializeApp() {
        try {
            statusBarUISet("Status: Initializing DB...");
            db = new GDB.GDB("cDiscuss-DB");

            db = await gdb("rbacChatAppDB", {
                rtc: true,
                sm: {
                    superAdmins: SUPERADMIN_ADDRESSES,
                    customRoles: CHAT_APP_ROLES,
                }
            });
            sm = db.sm

            statusBarUISet("Status: DB Ready. Initializing Security Context...");

            sm.setSecurityStateChangeCallback(updateState);

            // Trigger initial UI update based on current state (e.g. from silent WebAuthn login)
            // The callback itself will be called by createSecurityContext, but to be safe:
            const initialState = getInitialState();

            updateState(initialState);
            // Do NOT trigger interactive WebAuthn login here.
            // The Security Manager will attempt to silently resume a WebAuthn session during its initialization
            // when the last session used WebAuthn and a valid registration exists on this device.
            // Keep the "Login with WebAuthn" button so the user can start an interactive login when desired.

        } catch (error) {
            console.error("Initialization failed:", error);
            statusBarUISet(`Error: ${error.message}`);
            throw error;
        }
    }

    function statusBarUISet(text) {
        chrome.runtime.sendMessage({
            action: "statusBarUISet",
            text: text
        });
    }

    function displayMessage({
        id,
        value,
        action
    }) {
        chrome.runtime.sendMessage({
            action: "displayMessage",
            myData: {
                id: id,
                value: value,
                action: action
            },
            currentUserRole: currentUserRole,
            isSuperadminAddress: isSuperadminAddress(currentUserAddress)
        });
    }

    function clearMessagesContainer(pageHash) {
        chrome.runtime.sendMessage({
            action: "clearMessagesContainer",
            hash: pageHash
        });
    }

    async function loadMessages(pageHash, unsubscribeObject, doThrow) {
        if (unsubscribeObject.unsubscribe !== null || unsubscribeObject.referenceCount < 1)
            return;
        clearMessagesContainer(pageHash);

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
            if (doThrow)
                throw error;
            else
                alert(`Failed to load messages: ${error.message}`);
        }
    }

    function getInitialState() {
        return {
            isActive: sm.isSecurityActive(),
            activeAddress: sm.getActiveEthAddress(),
            isWebAuthnProtected: sm.isCurrentSessionProtectedByWebAuthn(),
            hasVolatileIdentity: !!sm.getMnemonicForDisplayAfterRegistrationOrRecovery(), // Heuristic
            hasWebAuthnHardwareRegistration: sm.hasExistingWebAuthnRegistration()
        };
    }

    async function newExtensionTab(pageHash, newExtensionTabId, sendResponse) {
        try {
            extensionTabIdToPageHash.set(newExtensionTabId, pageHash);

            referenceSubscription(pageHash);

            // Start the application
            if (!!db && !!sm) {

                const initialState = getInitialState();

                updateUICall(initialState);

                if (sm.isSecurityActive() && pageHashToReferenceCountedUnsubscribe.has(pageHash)) {
                    const unsubscribeObject = pageHashToReferenceCountedUnsubscribe.get(pageHash);
                    if (unsubscribeObject.unsubscribe) { // Unsubscribe from previous listener if any. This way all tabs for the same page will have all messages re-renderd.
                        unsubscribeObject.unsubscribe();
                        unsubscribeObject.unsubscribe = null;
                    }
                    await loadMessages(pageHash, unsubscribeObject, true);
                }


            } else {
                await initializeApp();
            }
            sendResponse({});
        } catch (error) {
            console.error("Failed to register another tab or initialize extension:", error);
            sendResponse({
                error: `Failed to register another tab or initialize extension:  ${error.message}`
            });
        }
    }

    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        if (message.action === "extensionTabInit") {

            console.log("Received in tab:", message.myData);
            const pageHash = message.myData.urlHash;
            const newExtensionTabId = message.myData.newExtensionTabId;

            newExtensionTab(pageHash, newExtensionTabId, sendResponse);
            return true;
        } else if (message.action === "registerNew") {
            registerNew(sendResponse);
            return true;
        } else if (message.action === "protectWebAuthn") {
            protectWebAuthn(sendResponse);
            return true;
        } else if (message.action === "loginWebAuthn") {
            loginWebAuthn(sendResponse);
            return true;
        } else if (message.action === "loginMnemonic") {
            loginMnemonic(message.mnemonic, sendResponse);
            return true;
        } else if (message.action === "logout") {
            logout(sendResponse);
            return true;
        } else if (message.action === "sendMessage") {
            sendMessage(message.hash, message.text, sendResponse);
            return true;
        }
    });


    function referenceSubscription(pageHash) {
        let unsubscribeObject = null;
        if (pageHashToReferenceCountedUnsubscribe.has(pageHash)) {
            unsubscribeObject = pageHashToReferenceCountedUnsubscribe.get(pageHash);
            unsubscribeObject.referenceCount++;
        } else {
            unsubscribeObject = {
                referenceCount: 1,
                unsubscribe: null
            }
            pageHashToReferenceCountedUnsubscribe.set(pageHash, unsubscribeObject);
        }

    }


    function unreferenceSubscription(pageHash) {
        if (!pageHashToReferenceCountedUnsubscribe.has(pageHash))
            return;
        const unsubscribeObject = pageHashToReferenceCountedUnsubscribe.get(pageHash);
        unsubscribeObject.referenceCount--;

        if (unsubscribeObject.referenceCount > 0)
            return;

        if (unsubscribeObject.unsubscribe)
            unsubscribeObject.unsubscribe();
        pageHashToReferenceCountedUnsubscribe.delete(pageHash);
    }

    function forceRemoveAllSubscriptions() {
        pageHashToReferenceCountedUnsubscribe.forEach((val, key, map) => {
            val.referenceCount = 0;

            if (val.unsubscribe) {
                val.unsubscribe();
                val.unsubscribe = null;
            }
        });
        pageHashToReferenceCountedUnsubscribe.clear();
    }

    function forceUnsubscribeAll() {
        pageHashToReferenceCountedUnsubscribe.forEach((val, key, map) => {

            if (val.unsubscribe) {
                val.unsubscribe();
                val.unsubscribe = null;
            }
        });
    }


    chrome.tabs.onRemoved.addListener((tabId) => {
        if (extensionTabIdToPageHash.has(tabId)) {
            const pageHash = extensionTabIdToPageHash.get(tabId);
            extensionTabIdToPageHash.delete(tabId)
            unreferenceSubscription(pageHash);

            cleanupIfNoExtensionTabs();
        }

    });

    function cleanupIfNoExtensionTabs() {
        if (extensionTabIdToPageHash.size === 0) {
            forceRemoveAllSubscriptions();
        }
    }

})();