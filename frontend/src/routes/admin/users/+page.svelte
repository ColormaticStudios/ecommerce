<script lang="ts">
	import { getContext, untrack } from "svelte";
	import { type API } from "$lib/api";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPaginationControls from "$lib/admin/AdminPaginationControls.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import {
		createAdminPaginatedResource,
		formatAdminDateTime,
		replaceItemById,
	} from "$lib/admin/state.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import { type UserModel } from "$lib/models";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();
	const initialData = untrack(() => $state.snapshot(data));
	const api: API = getContext("api");
	let hasLoadError = $state(Boolean(initialData.errorMessage));

	const limitOptions = [10, 20, 50, 100];
	const {
		collection: users,
		notices,
		sync,
	} = createAdminPaginatedResource<UserModel>({
		initial: {
			items: initialData.users,
			page: initialData.userPage,
			totalPages: initialData.userTotalPages,
			limit: initialData.userLimit,
			total: initialData.userTotal,
		},
		loadErrorMessage: "Unable to load users.",
		loadPage: async ({ query, page, limit }) => {
			const response = await api.listUsers({
				page,
				limit,
				q: query || undefined,
			});
			hasLoadError = false;
			return response;
		},
		onLoadError: () => {
			hasLoadError = true;
		},
	});

	async function updateRole(userId: number, role: string) {
		notices.clear();
		try {
			const updated = await api.updateUserRole(userId, { role });
			users.items = replaceItemById(users.items, updated);
			notices.pushSuccess("User role updated.");
		} catch (err) {
			console.error(err);
			notices.pushError("Unable to update role.");
		}
	}

	$effect(() => {
		sync(
			{
				items: data.users,
				page: data.userPage,
				totalPages: data.userTotalPages,
				limit: data.userLimit,
				total: data.userTotal,
			},
			data.errorMessage
		);
		hasLoadError = Boolean(data.errorMessage);
	});
</script>

{#snippet userActions()}
	<AdminResourceActions
		searchPlaceholder="Search ID, username, email, role..."
		searchInputClass="w-72"
		bind:searchValue={users.query}
		onSearch={users.applySearch}
		onRefresh={users.refresh}
		searchRefreshing={users.loading}
		searchDisabled={users.loading}
	/>
{/snippet}

{#snippet userHeaderActions()}
	<AdminResourceActions countLabel={`${users.total} users`} />
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Users" actions={userHeaderActions} />

	<AdminPanel
		title="User Directory"
		meta={`${users.items.length} shown`}
		headerActions={userActions}
	>
		{#if hasLoadError}
			<p class="admin-empty-state admin-empty-state-error">Failed to load users.</p>
		{:else if users.loading && users.items.length === 0}
			<p class="admin-empty-state">Loading users...</p>
		{:else if users.items.length === 0 && users.hasSearch}
			<p class="admin-empty-state">No users match "{users.query}".</p>
		{:else if users.items.length === 0}
			<p class="admin-empty-state">No users found.</p>
		{:else}
			<div class="space-y-4">
				{#each users.items as user (user.id)}
					<div class="admin-list-item flex flex-wrap items-start justify-between gap-4 p-4 text-sm">
						<div class="space-y-1">
							<p class="flex items-center gap-2 font-semibold text-stone-950 dark:text-stone-50">
								<span>{user.name || user.username}</span>
								{#if user.role === "admin"}
									<Badge tone="info" title="Admin" aria-label="Admin user">
										<i class="bi bi-shield-fill-check mr-1"></i>
										Admin
									</Badge>
								{/if}
							</p>
							<p class="admin-detail">@{user.username} · {user.email}</p>
							<p class="admin-detail">ID {user.id} · Currency {user.currency}</p>
							<p class="admin-detail">
								Created {formatAdminDateTime(user.created_at)} · Updated {formatAdminDateTime(
									user.updated_at
								)}
							</p>
							<p class="admin-detail break-all">Subject {user.subject}</p>
							{#if user.deleted_at}
								<p class="text-xs font-semibold text-rose-600 dark:text-rose-300">
									Deleted {formatAdminDateTime(user.deleted_at)}
								</p>
							{/if}
						</div>
						<div class="flex items-center gap-3">
							<span class="admin-label"> Role </span>
							<Dropdown
								tone="admin"
								full={false}
								class="px-3 py-1 text-sm"
								value={user.role}
								onchange={(event) => updateRole(user.id, (event.target as HTMLSelectElement).value)}
							>
								<option value="customer">Customer</option>
								<option value="admin">Admin</option>
							</Dropdown>
						</div>
					</div>
				{/each}

				<AdminPaginationControls
					page={users.page}
					totalPages={users.totalPages}
					totalItems={users.total}
					limit={users.limit}
					{limitOptions}
					onLimitChange={users.updateLimit}
					onPrev={() => void users.changePage(users.page - 1)}
					onNext={() => void users.changePage(users.page + 1)}
				/>
			</div>
		{/if}
	</AdminPanel>
</section>

<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
