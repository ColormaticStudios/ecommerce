<script lang="ts">
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import type { CmsEntryVersion, CmsPublication, DeliveryRuleDraft } from "./delivery";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	type ScheduleStatus = "pending" | "active" | "completed" | "cancelled" | null;
	type ExperimentStatus = "draft" | "active" | "paused" | "completed";
	interface Props {
		selectedPageId: number | null;
		deliveryLoading?: boolean;
		deliverySaving?: boolean;
		deliverySaveDisabled?: boolean;
		scheduleEnabled: boolean;
		schedulePublishAt: string;
		scheduleExpiryEnabled: boolean;
		scheduleUnpublishAt: string;
		scheduleTimezone: string;
		scheduleStatus: ScheduleStatus;
		scheduleLastTransitionAt: string | null;
		deliveryPublications: CmsPublication[];
		deliveryRules: DeliveryRuleDraft[];
		experimentEnabled: boolean;
		experimentName: string;
		experimentStatus: ExperimentStatus;
		experimentStickyKey: "visitor" | "customer";
		experimentStartsAt: string;
		experimentEndsAt: string;
		controlVersionId: number | null;
		variantVersionId: number | null;
		controlAllocation: number;
		pageVersionOptions: CmsEntryVersion[];
		publishedVersionId: number | null;
		addDeliveryRule: () => void;
		toggleRuleDevice: (
			rule: DeliveryRuleDraft,
			device: DeliveryRuleDraft["devices"][number]
		) => void;
		savePageDelivery: () => void | Promise<void>;
	}

	let {
		selectedPageId,
		deliveryLoading = false,
		deliverySaving = false,
		deliverySaveDisabled = false,
		scheduleEnabled = $bindable(),
		schedulePublishAt = $bindable(),
		scheduleExpiryEnabled = $bindable(),
		scheduleUnpublishAt = $bindable(),
		scheduleTimezone = $bindable(),
		scheduleStatus,
		scheduleLastTransitionAt,
		deliveryPublications,
		deliveryRules = $bindable(),
		experimentEnabled = $bindable(),
		experimentName = $bindable(),
		experimentStatus = $bindable(),
		experimentStickyKey = $bindable(),
		experimentStartsAt = $bindable(),
		experimentEndsAt = $bindable(),
		controlVersionId = $bindable(),
		variantVersionId = $bindable(),
		controlAllocation = $bindable(),
		pageVersionOptions,
		publishedVersionId,
		addDeliveryRule,
		toggleRuleDevice,
		savePageDelivery,
	}: Props = $props();
</script>

<AdminPanel title="Delivery" class="mt-6">
	{#if selectedPageId === null}
		<AdminEmptyState>Save the page before configuring delivery.</AdminEmptyState>
	{:else if deliveryLoading}
		<AdminEmptyState>Loading delivery settings...</AdminEmptyState>
	{:else}
		<div class="space-y-8">
			<section aria-labelledby="delivery-schedule-heading">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<div class="flex items-center gap-2">
							<h3 id="delivery-schedule-heading" class="text-sm font-semibold">Schedule</h3>
							{#if scheduleStatus}
								<Badge
									tone={scheduleStatus === "active"
										? "success"
										: scheduleStatus === "cancelled"
											? "neutral"
											: "warning"}
									size="sm">{scheduleStatus}</Badge
								>
							{/if}
						</div>
						<p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
							Publish this draft automatically at a specific time.
						</p>
					</div>
					<label class="flex items-center gap-2 text-sm font-medium">
						<input
							class="size-4 accent-stone-900 dark:accent-stone-100"
							type="checkbox"
							bind:checked={scheduleEnabled}
						/>
						Scheduled
					</label>
				</div>
				{#if scheduleEnabled}
					<div class="mt-4 grid gap-4 md:grid-cols-3">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Publish at</span>
							<TextInput tone="admin" type="datetime-local" bind:value={schedulePublishAt} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Timezone</span>
							<TextInput tone="admin" bind:value={scheduleTimezone} placeholder="UTC" />
						</label>
						<div class="text-sm">
							<label class="mb-2 flex items-center gap-2 font-medium">
								<input
									class="size-4 accent-stone-900 dark:accent-stone-100"
									type="checkbox"
									bind:checked={scheduleExpiryEnabled}
								/>
								Auto-expire
							</label>
							{#if scheduleExpiryEnabled}
								<TextInput
									aria-label="Unpublish at"
									tone="admin"
									type="datetime-local"
									bind:value={scheduleUnpublishAt}
								/>
							{/if}
						</div>
					</div>
				{/if}
				{#if scheduleLastTransitionAt}
					<p class="mt-3 text-xs text-stone-500 dark:text-stone-400">
						Last transition {new Date(scheduleLastTransitionAt).toLocaleString()}
					</p>
				{/if}
			</section>

			<section
				class="border-t border-stone-200 pt-8 dark:border-stone-800"
				aria-labelledby="delivery-audience-heading"
			>
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<h3 id="delivery-audience-heading" class="text-sm font-semibold">Audiences</h3>
						<p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
							Visitors matching any enabled audience can view this page.
						</p>
					</div>
					<Button tone="admin" size="small" onclick={addDeliveryRule}>
						<i class="bi bi-plus-lg mr-1"></i>Add audience
					</Button>
				</div>
				{#if deliveryRules.length === 0}
					<p class="mt-4 text-sm text-stone-500 dark:text-stone-400">Visible to every visitor.</p>
				{:else}
					<div
						class="mt-4 divide-y divide-stone-200 rounded-lg border border-stone-200 dark:divide-stone-800 dark:border-stone-800"
					>
						{#each deliveryRules as rule, index (rule.id)}
							<div class="p-4">
								<div class="mb-4 flex items-center justify-between gap-3">
									<label class="flex items-center gap-2 text-sm font-medium">
										<input
											class="size-4 accent-stone-900 dark:accent-stone-100"
											type="checkbox"
											bind:checked={rule.enabled}
										/>
										Audience {index + 1}
									</label>
									<IconButton
										tone="admin"
										variant="danger"
										outlined={true}
										size="sm"
										aria-label={`Remove audience ${index + 1}`}
										title="Remove audience"
										onclick={() =>
											(deliveryRules = deliveryRules.filter((item) => item.id !== rule.id))}
									>
										<i class="bi bi-trash"></i>
									</IconButton>
								</div>
								<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
									<label class="block text-sm"
										><span class="mb-1 block font-medium">Countries or markets</span><TextInput
											tone="admin"
											bind:value={rule.markets}
											placeholder="US, CA"
										/></label
									>
									<label class="block text-sm"
										><span class="mb-1 block font-medium">Visitor type</span><Dropdown
											tone="admin"
											bind:value={rule.audience}
											><option value="all">Everyone</option><option value="guest">Guests</option
											><option value="authenticated">Signed-in customers</option></Dropdown
										></label
									>
									<label class="block text-sm"
										><span class="mb-1 block font-medium">Referring sites</span><TextInput
											tone="admin"
											bind:value={rule.referrers}
											placeholder="google.com"
										/></label
									>
									<label class="block text-sm"
										><span class="mb-1 block font-medium">Campaign sources</span><TextInput
											tone="admin"
											bind:value={rule.utmSources}
											placeholder="newsletter, social"
										/></label
									>
									<label class="block text-sm"
										><span class="mb-1 block font-medium">Customer segments</span><TextInput
											tone="admin"
											bind:value={rule.segments}
											placeholder="vip, wholesale"
										/></label
									>
									<fieldset class="text-sm">
										<legend class="mb-2 font-medium">Devices</legend>
										<div class="flex flex-wrap gap-3">
											{#each ["desktop", "mobile", "tablet"] as device (device)}<label
													class="flex items-center gap-2 capitalize"
													><input
														class="size-4 accent-stone-900 dark:accent-stone-100"
														type="checkbox"
														checked={rule.devices.includes(
															device as "desktop" | "mobile" | "tablet"
														)}
														onchange={() =>
															toggleRuleDevice(rule, device as "desktop" | "mobile" | "tablet")}
													/>{device}</label
												>{/each}
										</div>
									</fieldset>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</section>

			<section
				class="border-t border-stone-200 pt-8 dark:border-stone-800"
				aria-labelledby="delivery-experiment-heading"
			>
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<h3 id="delivery-experiment-heading" class="text-sm font-semibold">Experiment</h3>
						<p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
							Compare two saved page versions with sticky visitor assignment.
						</p>
					</div>
					<label class="flex items-center gap-2 text-sm font-medium"
						><input
							class="size-4 accent-stone-900 dark:accent-stone-100"
							type="checkbox"
							bind:checked={experimentEnabled}
						/>Enabled</label
					>
				</div>
				{#if experimentEnabled}
					<div class="mt-4 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
						<label class="block text-sm lg:col-span-2"
							><span class="mb-1 block font-medium">Experiment name</span><TextInput
								tone="admin"
								bind:value={experimentName}
							/></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Status</span><Dropdown
								tone="admin"
								bind:value={experimentStatus}
								><option value="draft">Draft</option><option value="active">Active</option><option
									value="paused">Paused</option
								><option value="completed">Completed</option></Dropdown
							></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Keep assignment by</span><Dropdown
								tone="admin"
								bind:value={experimentStickyKey}
								><option value="visitor">Visitor</option><option value="customer"
									>Customer account</option
								></Dropdown
							></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Starts at</span><TextInput
								tone="admin"
								type="datetime-local"
								bind:value={experimentStartsAt}
							/></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Ends at (optional)</span><TextInput
								tone="admin"
								type="datetime-local"
								bind:value={experimentEndsAt}
							/></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Control version</span><Dropdown
								tone="admin"
								bind:value={controlVersionId}
								><option value={null}>Select version</option
								>{#each pageVersionOptions as version (version.id)}<option value={version.id}
										>Version {version.version_number}{version.id === publishedVersionId
											? " (published)"
											: " (draft)"}</option
									>{/each}</Dropdown
							></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Variant version</span><Dropdown
								tone="admin"
								bind:value={variantVersionId}
								><option value={null}>Select version</option
								>{#each pageVersionOptions as version (version.id)}<option value={version.id}
										>Version {version.version_number}{version.id === publishedVersionId
											? " (published)"
											: " (draft)"}</option
									>{/each}</Dropdown
							></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Control traffic (%)</span><NumberInput
								tone="admin"
								min={1}
								max={99}
								bind:value={controlAllocation}
							/></label
						>
						<div class="text-sm">
							<span class="mb-1 block font-medium">Variant traffic</span>
							<div
								class="rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 dark:border-stone-800 dark:bg-stone-900"
							>
								{100 - Number(controlAllocation || 0)}%
							</div>
						</div>
					</div>
				{/if}
			</section>

			{#if deliveryPublications.length > 0}
				<section
					class="border-t border-stone-200 pt-8 dark:border-stone-800"
					aria-labelledby="delivery-history-heading"
				>
					<h3 id="delivery-history-heading" class="text-sm font-semibold">Publication history</h3>
					<div
						class="mt-4 divide-y divide-stone-200 border-y border-stone-200 dark:divide-stone-800 dark:border-stone-800"
					>
						{#each deliveryPublications.slice(0, 5) as publication (publication.id)}
							<div class="flex flex-wrap items-center justify-between gap-2 py-3 text-sm">
								<div>
									<span class="font-medium">Published content</span>{#if publication.notes}<span
											class="ml-2 text-stone-500 dark:text-stone-400">{publication.notes}</span
										>{/if}
								</div>
								<time
									class="text-xs text-stone-500 dark:text-stone-400"
									datetime={publication.published_at}
									>{new Date(publication.published_at).toLocaleString()}</time
								>
							</div>
						{/each}
					</div>
				</section>
			{/if}

			<div class="flex justify-end border-t border-stone-200 pt-6 dark:border-stone-800">
				<Button
					tone="admin"
					variant="primary"
					onclick={() => void savePageDelivery()}
					disabled={deliverySaveDisabled}
				>
					<i class="bi bi-floppy mr-1"></i>{deliverySaving ? "Saving..." : "Save delivery"}
				</Button>
			</div>
		</div>
	{/if}
</AdminPanel>
