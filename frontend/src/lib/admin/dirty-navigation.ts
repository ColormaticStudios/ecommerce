export const ADMIN_DIRTY_NAVIGATION_CONTEXT = Symbol("admin-dirty-navigation");

const defaultDirtyNavigationMessage =
	"You have unsaved changes. Leave this section and discard them?";

export interface AdminDirtyNavigationState {
	dirty: boolean;
	message?: string;
}

export interface AdminDirtyNavigationController {
	readonly dirty: boolean;
	readonly message: string;
	update(token: symbol, state: AdminDirtyNavigationState): void;
	clear(token: symbol): void;
	shouldBlockNavigation(currentTarget: string | null, nextTarget: string | null): boolean;
	confirmNavigation(): boolean;
	allowNextNavigation(target: string | null): void;
}

interface StoredAdminDirtyNavigationState {
	dirty: boolean;
	message: string;
}

export function createAdminDirtyNavigationController(
	confirmNavigation: (message: string) => boolean
): AdminDirtyNavigationController {
	const registrations = new Map<symbol, StoredAdminDirtyNavigationState>();
	let allowedTarget: string | null = null;

	function activeState(): StoredAdminDirtyNavigationState | null {
		const states = Array.from(registrations.values());
		for (let index = states.length - 1; index >= 0; index -= 1) {
			if (states[index]?.dirty) {
				return states[index];
			}
		}
		return null;
	}

	return {
		get dirty() {
			return activeState()?.dirty ?? false;
		},
		get message() {
			return activeState()?.message ?? defaultDirtyNavigationMessage;
		},
		update(token, state) {
			registrations.set(token, {
				dirty: state.dirty,
				message: state.message?.trim() || defaultDirtyNavigationMessage,
			});
		},
		clear(token) {
			registrations.delete(token);
		},
		shouldBlockNavigation(currentTarget, nextTarget) {
			if (!this.dirty || !nextTarget || currentTarget === nextTarget) {
				return false;
			}
			if (allowedTarget === nextTarget) {
				allowedTarget = null;
				return false;
			}
			return true;
		},
		confirmNavigation() {
			return confirmNavigation(this.message);
		},
		allowNextNavigation(target) {
			allowedTarget = target;
		},
	};
}

export function toAdminNavigationTarget(url: URL | null | undefined) {
	if (!url) {
		return null;
	}
	return `${url.pathname}${url.search}`;
}
