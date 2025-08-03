// https://stackoverflow.com/questions/71080619/browser-extension-background-module#71081597

(async () => {

        import { GDB } from "/vendor/gdb.min.js"; // Adjust path according to your structure
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

        // --- RBAC Custom Roles ---
        const CHAT_APP_ROLES = {
          superadmin: { can: ["assignRole", "deleteAnyMessage", "write"], inherits: ["admin"] }, // Example
          admin: { can: ["deleteMessage"], inherits: ["user"] }, // Example
          user: { can: ["write", "readSelf"], inherits: ["guest"] }, // "write" allows sending messages
          guest: { can: ["read", "write", "sync"] }, // Default, can read public messages
        };
		
		
		async function updateState(securityState) {
            if (!securityState) {
				
				/* TODO
                statusBar.textContent = "Status: Security context not active.";
                authSection.classList.remove('hidden');
                chatSection.classList.add('hidden');
				*/
                currentUserAddress = null;
                return;
            }

            currentUserAddress = securityState.activeAddress;
            let statusText = `Status: ${securityState.isActive ? `Logged in as ${securityState.activeAddress.substring(0,10)}...` : 'Logged out.'}`;
            statusText += ` | WebAuthn Active: ${securityState.isWebAuthnProtected}`;
            statusText += ` | WebAuthn Registered Here: ${securityState.hasWebAuthnHardwareRegistration}`;
            // TODO statusBar.textContent = statusText;

            // TODO btnLoginWebAuthn.disabled = !securityState.hasWebAuthnHardwareRegistration;

            if (securityState.isActive) {
				for (let i = 0; i < SUPERADMIN_ADDRESSES.length; i++)
				{
			        await rbac.assignRole(SUPERADMIN_ADDRESSES[i], 'superadmin').catch((err) => {
                        throw new Error("assign superadmin role fail:" + err.message);
				    });
				}
				
                /* TODO authSection.classList.add('hidden');
                chatSection.classList.remove('hidden');
                newIdentityInfo.classList.add('hidden'); // Hide registration info if logged in
			   */
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
		
})();