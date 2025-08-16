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
                        if (val.unsubscribe)
                        { // Unsubscribe from previous listener if any
                          val.unsubscribe();
                          val.unsubscribe = null;
                        }
                        loadMessages(key, val);
                    });

                } else {
                    forceUnsubscribeAll();
                }
            }

            function updateUICall(securityState) {
                chrome.runtime.sendMessage({
                    action "updateUI",
                    securityState: securityState
                });
            }


            // --- IDENTITY MANAGEMENT HANDLERS ---
            async function registerNew(sendResponse)
            {
                try {
                    volatileIdentity = await rbac.startNewUserRegistration();
                    if (volatileIdentity) {
                        sendResponse({address: volatileIdentity.address, mnemonic: volatileIdentity.mnemonic});
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



            // --- INITIALIZATION ---
            async function initializeApp() {
                try {
                    statusBarUISet("Status: Initializing DB...");
                    db = new GDB("cDiscuss-DB");

                    statusBarUISet("Status: DB Ready. Initializing Security Context...");

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
                    statusBarUISet(`Error: ${error.message}`);
                    alert(`Initialization Error: ${error.message}`);
                }
            }

            function statusBarUISet(text) {
                chrome.runtime.sendMessage({
                        action "statusBarUISet",
                        text: text
                    }

                    function displayMessage({
                        id,
                        value,
                        action
                    }) {
                        chrome.runtime.sendMessage({
                            action "displayMessage",
                            myData: {
                                id: id,
                                value: value,
                                action: action
                            }
                        });
                    }

                    function clearMessagesContainer(pageHash) {
                        chrome.runtime.sendMessage({
                            action "clearMessagesContainer",
                            hash: pageHash
                        });
                    }

                    async function loadMessages(pageHash, unsubscribeObject) {
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
                                    loadMessages(pageHash, unsubscribeObject);
                                }

                            } else
                                initializeApp();

                        }
                        else if (message.action === "registerNew")
                        {
                          registerNew(sendResponse);
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

                    function forceUnsubscribeAll() {
                        pageHashToReferenceCountedUnsubscribe.forEach((key, val, map) => {

                            if (val.unsubscribe)
                            {
                                val.unsubscribe();
                                val.uunsubscribe = null;
                            }
                        });
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
                        }
                    }

                })();