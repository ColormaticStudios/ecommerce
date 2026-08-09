<script lang="ts">
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	type CmsGovernance = components["schemas"]["CmsGovernance"];
	type CmsOperations = components["schemas"]["CmsOperations"];
	type GovernanceRole = CmsGovernance["roles"][number];

	interface Props {
		governance: CmsGovernance;
		governanceSaving?: boolean;
		operations: CmsOperations | null;
		operationsLoading?: boolean;
		onAddRole: () => void;
		onUpdateRole: (index: number, patch: Partial<GovernanceRole>) => void;
		onRemoveRole: (index: number) => void;
		onSaveGovernance: () => void | Promise<void>;
		onRefreshOperations: () => void | Promise<void>;
		onRetryInvalidation: (id: number) => void | Promise<void>;
	}

	let {
		governance = $bindable(),
		governanceSaving = false,
		operations,
		operationsLoading = false,
		onAddRole,
		onUpdateRole,
		onRemoveRole,
		onSaveGovernance,
		onRefreshOperations,
		onRetryInvalidation,
	}: Props = $props();
</script>

<div class="mt-6 grid gap-6 xl:grid-cols-[1fr_1fr]">
	<AdminPanel title="Governance">
		<div class="space-y-5">
			<label class="flex items-center gap-3 text-sm">
				<input
					type="checkbox"
					class="h-4 w-4 rounded border-stone-300"
					bind:checked={governance.approval_required}
				/>
				<span>Require approval before publishing</span>
			</label>
			<label class="block text-sm">
				<span class="mb-1 block font-medium">Invalidation webhook URL</span>
				<TextInput
					tone="admin"
					bind:value={governance.invalidation_webhook_url}
					placeholder="https://example.com/cms/invalidate"
				/>
			</label>
			<section aria-labelledby="cms-roles-heading">
				<div class="mb-3 flex items-center justify-between gap-3">
					<h3 id="cms-roles-heading" class="text-sm font-semibold">CMS roles</h3>
					<Button tone="admin" variant="regular" size="small" onclick={onAddRole}>
						<i class="bi bi-plus-lg mr-1"></i>Add role
					</Button>
				</div>
				{#if governance.roles.length === 0}
					<AdminEmptyState>No explicit role assignments.</AdminEmptyState>
				{:else}
					<div class="space-y-3">
						{#each governance.roles as role, index (index)}
							<AdminSurface
								as="div"
								variant="muted"
								class="grid gap-3 p-3 md:grid-cols-[1fr_10rem_auto]"
							>
								<TextInput
									tone="admin"
									value={role.subject}
									placeholder="user@example.com"
									oninput={(event) =>
										onUpdateRole(index, {
											subject: (event.currentTarget as HTMLInputElement).value,
										})}
								/>
								<Dropdown
									tone="admin"
									value={role.role}
									onchange={(event) =>
										onUpdateRole(index, {
											role: (event.currentTarget as HTMLSelectElement)
												.value as GovernanceRole["role"],
										})}
								>
									<option value="author">Author</option><option value="editor">Editor</option
									><option value="publisher">Publisher</option>
								</Dropdown>
								<IconButton
									tone="admin"
									variant="danger"
									outlined={true}
									size="sm"
									aria-label="Remove role"
									title="Remove role"
									onclick={() => onRemoveRole(index)}><i class="bi bi-trash"></i></IconButton
								>
							</AdminSurface>
						{/each}
					</div>
				{/if}
			</section>
			<div class="flex justify-end">
				<Button
					tone="admin"
					variant="primary"
					onclick={onSaveGovernance}
					disabled={governanceSaving}
				>
					<i class="bi bi-floppy mr-1"></i>{governanceSaving ? "Saving..." : "Save governance"}
				</Button>
			</div>
		</div>
	</AdminPanel>

	<AdminPanel title="Operations">
		<div class="space-y-5">
			<div class="flex items-center justify-between gap-3">
				<div class="grid grid-cols-2 gap-3 text-sm">
					<AdminSurface as="div" variant="muted" class="p-4"
						><div class="text-2xl font-semibold">{operations?.pending_schedules ?? 0}</div>
						<div class="text-stone-500 dark:text-stone-400">Pending schedules</div></AdminSurface
					>
					<AdminSurface as="div" variant="muted" class="p-4"
						><div class="text-2xl font-semibold">{operations?.active_experiments ?? 0}</div>
						<div class="text-stone-500 dark:text-stone-400">Active experiments</div></AdminSurface
					>
				</div>
				<Button
					tone="admin"
					variant="regular"
					onclick={onRefreshOperations}
					disabled={operationsLoading}
				>
					<i class="bi bi-arrow-clockwise mr-1"></i>{operationsLoading
						? "Refreshing..."
						: "Refresh"}
				</Button>
			</div>
			<section aria-labelledby="cms-invalidations-heading">
				<h3 id="cms-invalidations-heading" class="mb-3 text-sm font-semibold">Invalidations</h3>
				{#if !operations || operations.invalidations.length === 0}
					<AdminEmptyState>No invalidation events.</AdminEmptyState>
				{:else}
					<div class="space-y-3">
						{#each operations.invalidations as event (event.id)}
							<AdminListItem class="flex items-center justify-between gap-3 p-4">
								<div class="min-w-0">
									<div class="font-medium">{event.reason}</div>
									<div class="truncate text-xs text-stone-500">
										Entry {event.entry_id} · attempts {event.attempts}{event.last_error
											? ` · ${event.last_error}`
											: ""}
									</div>
								</div>
								<div class="flex items-center gap-2">
									<Badge
										tone={event.status === "sent"
											? "success"
											: event.status === "failed"
												? "danger"
												: "neutral"}
										size="sm">{event.status}</Badge
									>
									<Button
										tone="admin"
										variant="regular"
										size="small"
										onclick={() => onRetryInvalidation(event.id)}
										disabled={event.status === "pending"}>Retry</Button
									>
								</div>
							</AdminListItem>
						{/each}
					</div>
				{/if}
			</section>
		</div>
	</AdminPanel>
</div>
