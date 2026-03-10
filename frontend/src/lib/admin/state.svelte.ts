import { getContext, onDestroy } from "svelte";
import {
	ADMIN_DIRTY_NAVIGATION_CONTEXT,
	type AdminDirtyNavigationController,
} from "$lib/admin/dirty-navigation";

export type AdminNoticeTone = "success" | "error" | null;

export interface AdminPaginatedSnapshot<T> {
	items: T[];
	page: number;
	totalPages: number;
	limit: number;
	total: number;
	query?: string;
}

export interface AdminPaginatedLoadParams {
	query: string;
	page: number;
	limit: number;
}

export interface AdminPaginatedResult<T> {
	items: T[];
	page?: number;
	totalPages: number;
	total: number;
}

interface AdminPaginatedApiResponse<T> {
	data: T[];
	pagination: {
		page: number;
		total_pages: number;
		total: number;
	};
}

interface AdminPaginatedCollectionOptions<T> {
	initial: AdminPaginatedSnapshot<T>;
	loadPage: (params: AdminPaginatedLoadParams) => Promise<AdminPaginatedResult<T>>;
	onLoadError?: (error: unknown) => void;
	beforeLoad?: () => void;
	afterLoad?: (items: T[]) => Promise<void> | void;
	fallbackLimit?: number;
}

interface AdminPaginatedResourceOptions<T> {
	initial: AdminPaginatedSnapshot<T>;
	loadPage: (params: AdminPaginatedLoadParams) => Promise<AdminPaginatedApiResponse<T>>;
	loadErrorMessage: string;
	onLoadError?: (error: unknown) => void;
	afterLoad?: (items: T[]) => Promise<void> | void;
	fallbackLimit?: number;
}

interface AdminSavePromptOptions {
	onSaveError?: (error: unknown) => void;
	navigationMessage?: string;
}

export function createAdminNotices() {
	let message = $state("");
	let tone = $state<AdminNoticeTone>(null);

	function clear() {
		message = "";
		tone = null;
	}

	function set(nextTone: Exclude<AdminNoticeTone, null>, nextMessage: string) {
		tone = nextTone;
		message = nextMessage;
	}

	function pushError(nextMessage: string) {
		if (!nextMessage.trim()) {
			return;
		}
		set("error", nextMessage);
	}

	function pushSuccess(nextMessage: string) {
		if (!nextMessage.trim()) {
			return;
		}
		set("success", nextMessage);
	}

	function setError(nextMessage: string) {
		if (!nextMessage.trim()) {
			if (tone === "error") {
				clear();
			}
			return;
		}
		set("error", nextMessage);
	}

	function setSuccess(nextMessage: string) {
		if (!nextMessage.trim()) {
			if (tone === "success") {
				clear();
			}
			return;
		}
		set("success", nextMessage);
	}

	return {
		get message() {
			return message;
		},
		set message(value: string) {
			message = value;
		},
		get tone() {
			return tone;
		},
		set tone(value: AdminNoticeTone) {
			tone = value;
		},
		clear,
		set,
		pushError,
		pushSuccess,
		setError,
		setSuccess,
	};
}

export function createAdminSavePrompt(options: AdminSavePromptOptions = {}) {
	let dirty = $state(false);
	let blocked = $state(false);
	let saving = $state(false);
	let saveAction = $state<(() => Promise<void>) | null>(null);
	const dirtyNavigation = getContext<AdminDirtyNavigationController | undefined>(
		ADMIN_DIRTY_NAVIGATION_CONTEXT
	);
	const dirtyNavigationToken = Symbol("admin-save-prompt");

	const canSave = $derived(saveAction !== null && !saving && !blocked);

	if (dirtyNavigation) {
		$effect(() => {
			dirtyNavigation.update(dirtyNavigationToken, {
				dirty,
				message: options.navigationMessage,
			});
		});

		onDestroy(() => {
			dirtyNavigation.clear(dirtyNavigationToken);
		});
	}

	async function save() {
		if (!saveAction || saving || blocked) {
			return;
		}

		saving = true;
		try {
			await saveAction();
		} catch (error) {
			console.error(error);
			options.onSaveError?.(error);
		} finally {
			saving = false;
		}
	}

	return {
		get dirty() {
			return dirty;
		},
		set dirty(value: boolean) {
			dirty = value;
		},
		get blocked() {
			return blocked;
		},
		set blocked(value: boolean) {
			blocked = value;
		},
		get saving() {
			return saving;
		},
		get saveAction() {
			return saveAction;
		},
		set saveAction(value: (() => Promise<void>) | null) {
			saveAction = value;
		},
		get canSave() {
			return canSave;
		},
		save,
	};
}

export function createAdminPaginatedCollection<T>(options: AdminPaginatedCollectionOptions<T>) {
	let items = $state<T[]>(options.initial.items);
	let query = $state(options.initial.query ?? "");
	let page = $state(options.initial.page);
	let totalPages = $state(options.initial.totalPages);
	let limit = $state(options.initial.limit);
	let total = $state(options.initial.total);
	let loading = $state(false);

	const hasSearch = $derived(query.trim().length > 0);

	function sync(snapshot: AdminPaginatedSnapshot<T>) {
		items = snapshot.items;
		page = snapshot.page;
		totalPages = snapshot.totalPages;
		limit = snapshot.limit;
		total = snapshot.total;
		loading = false;
		if (snapshot.query !== undefined) {
			query = snapshot.query;
		}
	}

	async function load() {
		const nextPage = page;
		const nextLimit = limit;

		loading = true;
		options.beforeLoad?.();
		try {
			const result = await options.loadPage({
				query: query.trim(),
				page: nextPage,
				limit: nextLimit,
			});
			items = result.items;
			page = Math.max(1, result.page ?? nextPage);
			totalPages = Math.max(1, result.totalPages);
			total = result.total;
			await options.afterLoad?.(result.items);
		} catch (error) {
			options.onLoadError?.(error);
		} finally {
			loading = false;
		}
	}

	async function changePage(nextPage: number) {
		if (nextPage < 1 || nextPage > totalPages || nextPage === page) {
			return;
		}
		page = nextPage;
		await load();
	}

	function updateLimit(nextLimit: number) {
		limit = Number.isNaN(nextLimit) ? (options.fallbackLimit ?? 20) : nextLimit;
		page = 1;
		void load();
	}

	function applySearch() {
		page = 1;
		void load();
	}

	function refresh() {
		void load();
	}

	return {
		get items() {
			return items;
		},
		set items(value: T[]) {
			items = value;
		},
		get query() {
			return query;
		},
		set query(value: string) {
			query = value;
		},
		get page() {
			return page;
		},
		set page(value: number) {
			page = value;
		},
		get totalPages() {
			return totalPages;
		},
		set totalPages(value: number) {
			totalPages = value;
		},
		get limit() {
			return limit;
		},
		set limit(value: number) {
			limit = value;
		},
		get total() {
			return total;
		},
		set total(value: number) {
			total = value;
		},
		get loading() {
			return loading;
		},
		get hasSearch() {
			return hasSearch;
		},
		sync,
		load,
		refresh,
		changePage,
		updateLimit,
		applySearch,
	};
}

export function createAdminPaginatedResource<T>(options: AdminPaginatedResourceOptions<T>) {
	const notices = createAdminNotices();
	const collection = createAdminPaginatedCollection<T>({
		initial: options.initial,
		beforeLoad: notices.clear,
		loadPage: async ({ page, ...params }) => {
			const response = await options.loadPage({ page, ...params });
			return mapAdminPaginatedResponse(response, page);
		},
		onLoadError: (error) => {
			console.error(error);
			notices.pushError(options.loadErrorMessage);
			options.onLoadError?.(error);
		},
		afterLoad: options.afterLoad,
		fallbackLimit: options.fallbackLimit,
	});

	function sync(snapshot: AdminPaginatedSnapshot<T>, errorMessage?: string) {
		collection.sync(snapshot);
		if (errorMessage) {
			notices.pushError(errorMessage);
		}
	}

	return {
		collection,
		notices,
		sync,
	};
}

interface AdminRecord {
	id: number | string;
}

export function replaceItemById<T extends AdminRecord>(items: T[], updated: T) {
	return items.map((item) => (item.id === updated.id ? updated : item));
}

export function upsertItemById<T extends AdminRecord>(items: T[], nextItem: T) {
	const index = items.findIndex((item) => item.id === nextItem.id);
	if (index === -1) {
		return [nextItem, ...items];
	}

	const nextItems = [...items];
	nextItems[index] = nextItem;
	return nextItems;
}

export function removeItemById<T extends AdminRecord>(items: T[], itemId: T["id"]) {
	return items.filter((item) => item.id !== itemId);
}

export function formatAdminDateTime(value: Date) {
	return value.toLocaleString();
}

function mapAdminPaginatedResponse<T>(
	response: AdminPaginatedApiResponse<T>,
	requestedPage: number
): AdminPaginatedResult<T> {
	return {
		items: response.data,
		page: Math.max(1, response.pagination.page ?? requestedPage),
		totalPages: Math.max(1, response.pagination.total_pages),
		total: response.pagination.total,
	};
}
