<script lang="ts">
	import type { Snippet } from "svelte";
	import type { components } from "$lib/api/generated/openapi";
	import type { CmsContentBlock } from "$lib/cms";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import { hasBlockingCmsBlocks, type CmsEditableBlock } from "./blocks";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	type CmsLocale = components["schemas"]["CmsLocale"];
	type CmsPageVariant = components["schemas"]["CmsPageVariant"];
	interface Props {
		selectedPageId: number | null;
		cmsLocales: CmsLocale[];
		localeSaving?: boolean;
		pageVariants: CmsPageVariant[];
		selectedVariantId: number | null;
		currentVariant: CmsPageVariant | null;
		variantLocale: string;
		variantMarket: string;
		variantPath: string;
		variantSlug: string;
		variantTitle: string;
		variantBlocks: CmsEditableBlock[];
		variantSaving?: boolean;
		variantComment: string;
		selectedPageSectionType: CmsContentBlock["type"];
		pageSectionOptions: Array<{ id: CmsContentBlock["type"]; label: string }>;
		blockEditor: Snippet<[CmsEditableBlock[], "page" | "global" | "variant"]>;
		addLocale: () => void;
		saveLocales: () => void | Promise<void>;
		newPageVariant: () => void;
		selectPageVariant: (variant: CmsPageVariant) => void;
		addBlock: (target: "page" | "global" | "variant", type: CmsContentBlock["type"]) => void;
		savePageVariant: () => void | Promise<void>;
		transitionPageVariant: (
			action: "submit" | "request_changes" | "approve" | "publish"
		) => void | Promise<void>;
		removePageVariant: () => void | Promise<void>;
	}

	let {
		selectedPageId,
		cmsLocales = $bindable(),
		localeSaving = false,
		pageVariants,
		selectedVariantId,
		currentVariant,
		variantLocale = $bindable(),
		variantMarket = $bindable(),
		variantPath = $bindable(),
		variantSlug = $bindable(),
		variantTitle = $bindable(),
		variantBlocks = $bindable(),
		variantSaving = false,
		variantComment = $bindable(),
		selectedPageSectionType = $bindable(),
		pageSectionOptions,
		blockEditor,
		addLocale,
		saveLocales,
		newPageVariant,
		selectPageVariant,
		addBlock,
		savePageVariant,
		transitionPageVariant,
		removePageVariant,
	}: Props = $props();
</script>

<AdminPanel title="Languages and markets" class="mt-6">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<p class="text-sm text-stone-500 dark:text-stone-400">
			Configure storefront languages and deterministic fallback order.
		</p>
		<Button tone="admin" size="small" onclick={addLocale}>
			<i class="bi bi-plus-lg mr-1"></i>Add language
		</Button>
	</div>
	<div
		class="mt-4 divide-y divide-stone-200 border-y border-stone-200 dark:divide-stone-800 dark:border-stone-800"
	>
		{#each cmsLocales as locale, index (index)}
			<div class="grid gap-3 py-4 md:grid-cols-[9rem_1fr_11rem_auto] md:items-end">
				<label class="block text-sm">
					<span class="mb-1 block font-medium">Locale</span>
					<TextInput tone="admin" bind:value={locale.code} placeholder="fr-CA" />
				</label>
				<label class="block text-sm">
					<span class="mb-1 block font-medium">Display name</span>
					<TextInput tone="admin" bind:value={locale.name} placeholder="French (Canada)" />
				</label>
				<label class="block text-sm">
					<span class="mb-1 block font-medium">Fallback</span>
					<Dropdown tone="admin" bind:value={locale.fallback_locale}>
						<option value={null}>No fallback</option>
						{#each cmsLocales.filter((candidate) => candidate !== locale && candidate.code) as candidate (candidate.code)}
							<option value={candidate.code}>{candidate.code}</option>
						{/each}
					</Dropdown>
				</label>
				<div class="flex flex-wrap items-center gap-4 pb-2 text-sm">
					<label class="flex items-center gap-2">
						<input
							class="size-4 accent-stone-900 dark:accent-stone-100"
							type="checkbox"
							bind:checked={locale.enabled}
						/>
						Enabled
					</label>
					<label class="flex items-center gap-2">
						<input
							class="size-4 accent-stone-900 dark:accent-stone-100"
							type="radio"
							name="default-cms-locale"
							checked={locale.is_default}
							onchange={() =>
								(cmsLocales = cmsLocales.map((item) => ({
									...item,
									is_default: item === locale,
								})))}
						/>
						Default
					</label>
					<IconButton
						tone="admin"
						variant="danger"
						outlined={true}
						size="sm"
						aria-label={`Remove ${locale.name || "language"}`}
						title="Remove language"
						disabled={locale.is_default}
						onclick={() => (cmsLocales = cmsLocales.filter((_, itemIndex) => itemIndex !== index))}
					>
						<i class="bi bi-trash"></i>
					</IconButton>
				</div>
			</div>
		{/each}
	</div>
	<div class="mt-4 flex justify-end">
		<Button
			tone="admin"
			variant="primary"
			disabled={localeSaving}
			onclick={() => void saveLocales()}
		>
			<i class="bi bi-floppy mr-1"></i>{localeSaving ? "Saving..." : "Save languages"}
		</Button>
	</div>
</AdminPanel>

<AdminPanel title="Localized page variants" class="mt-6">
	{#if selectedPageId === null}
		<AdminEmptyState>Save the page before adding localized variants.</AdminEmptyState>
	{:else}
		<div class="grid gap-6 lg:grid-cols-[15rem_minmax(0,1fr)]">
			<div>
				<Button tone="admin" size="small" class="mb-3 w-full" onclick={newPageVariant}>
					<i class="bi bi-plus-lg mr-1"></i>New variant
				</Button>
				<div class="space-y-2">
					{#each pageVariants as variant (variant.id)}
						<AdminListItem
							as="button"
							active={variant.id === selectedVariantId}
							interactive={variant.id !== selectedVariantId}
							class="flex items-center justify-between gap-2 p-3"
							onclick={() => selectPageVariant(variant)}
						>
							<span class="min-w-0 text-left">
								<span class="block truncate text-sm font-medium"
									>{variant.locale}{variant.market ? ` / ${variant.market}` : ""}</span
								>
								<span class="block truncate text-xs text-stone-500">{variant.path}</span>
							</span>
							<Badge
								tone={variant.status === "published"
									? "success"
									: variant.status === "approved"
										? "neutral"
										: "warning"}
								size="sm">{variant.status.replace("_", " ")}</Badge
							>
						</AdminListItem>
					{/each}
				</div>
			</div>
			<div class="min-w-0">
				<div class="grid gap-4 md:grid-cols-2">
					<label class="block text-sm"
						><span class="mb-1 block font-medium">Language</span><Dropdown
							tone="admin"
							bind:value={variantLocale}
							><option value="">Choose language</option
							>{#each cmsLocales.filter((locale) => locale.enabled && !locale.is_default) as locale (locale.code)}<option
									value={locale.code}>{locale.name}</option
								>{/each}</Dropdown
						></label
					>
					<label class="block text-sm"
						><span class="mb-1 block font-medium">Market override</span><TextInput
							tone="admin"
							bind:value={variantMarket}
							placeholder="Optional, for example CA"
						/></label
					>
					<label class="block text-sm"
						><span class="mb-1 block font-medium">Title</span><TextInput
							tone="admin"
							bind:value={variantTitle}
						/></label
					>
					<label class="block text-sm"
						><span class="mb-1 block font-medium">Localized path</span><TextInput
							tone="admin"
							bind:value={variantPath}
							placeholder="/fr/livraison"
						/></label
					>
					<label class="block text-sm"
						><span class="mb-1 block font-medium">Slug</span><TextInput
							tone="admin"
							bind:value={variantSlug}
						/></label
					>
				</div>
				<div
					class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-stone-200 pt-5 dark:border-stone-800"
				>
					<h3 class="text-sm font-semibold">Localized sections</h3>
					<div class="flex gap-2">
						<Dropdown tone="admin" full={false} bind:value={selectedPageSectionType}
							>{#each pageSectionOptions as option (option.id)}<option value={option.id}
									>{option.label}</option
								>{/each}</Dropdown
						>
						<Button
							tone="admin"
							size="small"
							onclick={() => addBlock("variant", selectedPageSectionType)}
							><i class="bi bi-plus-lg mr-1"></i>Add</Button
						>
					</div>
				</div>
				<div class="mt-4">{@render blockEditor(variantBlocks, "variant")}</div>
				<div
					class="mt-5 flex flex-wrap items-end gap-3 border-t border-stone-200 pt-5 dark:border-stone-800"
				>
					<label class="min-w-64 flex-1 text-sm"
						><span class="mb-1 block font-medium">Review note</span><TextInput
							tone="admin"
							bind:value={variantComment}
							placeholder="Context for the next reviewer"
						/></label
					>
					<Button
						tone="admin"
						variant="primary"
						disabled={variantSaving || !variantLocale || !variantTitle || !variantPath}
						onclick={() => void savePageVariant()}
						>{variantSaving
							? "Saving..."
							: selectedVariantId === null
								? "Create draft"
								: "Save draft"}</Button
					>
					{#if currentVariant?.status === "draft" || currentVariant?.status === "changes_requested"}<Button
							tone="admin"
							onclick={() => void transitionPageVariant("submit")}>Submit for review</Button
						>{/if}
					{#if currentVariant?.status === "in_review"}<Button
							tone="admin"
							onclick={() => void transitionPageVariant("request_changes")}>Request changes</Button
						><Button
							tone="admin"
							variant="success"
							onclick={() => void transitionPageVariant("approve")}>Approve</Button
						>{/if}
					{#if currentVariant?.status === "approved"}<Button
							tone="admin"
							variant="success"
							disabled={hasBlockingCmsBlocks(variantBlocks)}
							onclick={() => void transitionPageVariant("publish")}>Publish</Button
						>{/if}
					{#if selectedVariantId !== null}<IconButton
							tone="admin"
							variant="danger"
							outlined={true}
							aria-label="Delete localized variant"
							title="Delete localized variant"
							onclick={() => void removePageVariant()}><i class="bi bi-trash"></i></IconButton
						>{/if}
				</div>
			</div>
		</div>
	{/if}
</AdminPanel>
