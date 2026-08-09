<script lang="ts">
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import { cmsMediaURL } from "$lib/cms";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	type Robots = "index_follow" | "noindex_follow" | "index_nofollow" | "noindex_nofollow";
	type TwitterCard = "summary" | "summary_large_image";
	type JsonLdType =
		| ""
		| "WebPage"
		| "FAQPage"
		| "BreadcrumbList"
		| "Organization"
		| "WebSite"
		| "Product";

	interface Props {
		selectedPageId: number | null;
		pagePath: string;
		seoLoading?: boolean;
		seoSaving?: boolean;
		seoTitle: string;
		seoDescription: string;
		seoCanonicalURL: string;
		seoRobots: Robots;
		seoOGTitle: string;
		seoOGDescription: string;
		seoOGImageMediaID: string;
		seoTwitterCard: TwitterCard;
		seoTwitterTitle: string;
		seoTwitterDescription: string;
		seoTwitterImageMediaID: string;
		seoJSONLDType: JsonLdType;
		seoJSONLDName: string;
		seoIssues?: string[];
		uploadSEOMedia: (event: Event, target: "og" | "twitter") => void | Promise<void>;
		savePageSEO: () => void | Promise<void>;
	}

	let {
		selectedPageId,
		pagePath,
		seoLoading = false,
		seoSaving = false,
		seoTitle = $bindable(),
		seoDescription = $bindable(),
		seoCanonicalURL = $bindable(),
		seoRobots = $bindable(),
		seoOGTitle = $bindable(),
		seoOGDescription = $bindable(),
		seoOGImageMediaID = $bindable(),
		seoTwitterCard = $bindable(),
		seoTwitterTitle = $bindable(),
		seoTwitterDescription = $bindable(),
		seoTwitterImageMediaID = $bindable(),
		seoJSONLDType = $bindable(),
		seoJSONLDName = $bindable(),
		seoIssues = [],
		uploadSEOMedia,
		savePageSEO,
	}: Props = $props();
</script>

<AdminPanel title="Search and sharing" class="mt-6">
	{#if selectedPageId === null}
		<AdminEmptyState>Save the page before configuring SEO.</AdminEmptyState>
	{:else if seoLoading}
		<AdminEmptyState>Loading SEO settings...</AdminEmptyState>
	{:else}
		{#if seoIssues.length > 0}
			<div
				class="mb-6 space-y-1 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-100"
			>
				{#each seoIssues as issue (issue)}<p>{issue}</p>{/each}
			</div>
		{/if}
		<div class="grid gap-4 md:grid-cols-2">
			<label class="block text-sm"
				><span class="mb-1 block font-medium">Search title</span><TextInput
					tone="admin"
					bind:value={seoTitle}
				/><span class="mt-1 block text-xs text-stone-500">{seoTitle.length}/60</span></label
			>
			<label class="block text-sm"
				><span class="mb-1 block font-medium">Canonical URL</span><TextInput
					tone="admin"
					bind:value={seoCanonicalURL}
					placeholder={pagePath || "/page"}
				/></label
			>
			<label class="block text-sm md:col-span-2"
				><span class="mb-1 block font-medium">Meta description</span><TextArea
					tone="admin"
					class="min-h-24"
					bind:value={seoDescription}
				/><span class="mt-1 block text-xs text-stone-500">{seoDescription.length}/160</span></label
			>
			<label class="block text-sm"
				><span class="mb-1 block font-medium">Search engine access</span><Dropdown
					tone="admin"
					bind:value={seoRobots}
					><option value="index_follow">Index page and follow links</option><option
						value="noindex_follow">Hide page, follow links</option
					><option value="index_nofollow">Index page, ignore links</option><option
						value="noindex_nofollow">Hide page and ignore links</option
					></Dropdown
				></label
			>
		</div>

		<div class="mt-8 border-t border-stone-200 pt-6 dark:border-stone-800">
			<h3 class="text-sm font-semibold">Social sharing</h3>
			<div class="mt-4 grid gap-4 md:grid-cols-2">
				<label class="block text-sm"
					><span class="mb-1 block font-medium">Social title</span><TextInput
						tone="admin"
						bind:value={seoOGTitle}
					/></label
				>
				<label class="block text-sm"
					><span class="mb-1 block font-medium">Social description</span><TextInput
						tone="admin"
						bind:value={seoOGDescription}
					/></label
				>
				<div class="text-sm">
					<span class="mb-1 block font-medium">Social image</span><label
						class="flex min-h-20 cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 dark:border-stone-700"
						><input
							class="sr-only"
							type="file"
							accept="image/*"
							onchange={(event) => void uploadSEOMedia(event, "og")}
						/>{seoOGImageMediaID ? "Replace image" : "Upload image"}</label
					>{#if seoOGImageMediaID}<img
							class="mt-2 aspect-video w-full rounded-lg object-cover"
							src={cmsMediaURL(seoOGImageMediaID)}
							alt=""
						/>{/if}
				</div>
				<label class="block text-sm"
					><span class="mb-1 block font-medium">X card format</span><Dropdown
						tone="admin"
						bind:value={seoTwitterCard}
						><option value="summary">Compact</option><option value="summary_large_image"
							>Large image</option
						></Dropdown
					><span class="mt-3 mb-1 block font-medium">X title</span><TextInput
						tone="admin"
						bind:value={seoTwitterTitle}
					/><span class="mt-3 mb-1 block font-medium">X description</span><TextInput
						tone="admin"
						bind:value={seoTwitterDescription}
					/><span class="mt-3 mb-1 block font-medium">X image</span><span
						class="flex cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 px-3 py-2 dark:border-stone-700"
						><input
							class="sr-only"
							type="file"
							accept="image/*"
							onchange={(event) => void uploadSEOMedia(event, "twitter")}
						/>{seoTwitterImageMediaID ? "Replace image" : "Upload image"}</span
					></label
				>
			</div>
		</div>

		<div class="mt-8 border-t border-stone-200 pt-6 dark:border-stone-800">
			<h3 class="text-sm font-semibold">Structured data</h3>
			<div class="mt-4 grid gap-4 md:grid-cols-2">
				<label class="block text-sm"
					><span class="mb-1 block font-medium">Content type</span><Dropdown
						tone="admin"
						bind:value={seoJSONLDType}
						><option value="">None</option><option value="WebPage">Web page</option><option
							value="FAQPage">FAQ page</option
						><option value="BreadcrumbList">Breadcrumb list</option><option value="Organization"
							>Organization</option
						><option value="WebSite">Website</option><option value="Product">Product</option
						></Dropdown
					></label
				>
				{#if seoJSONLDType}<label class="block text-sm"
						><span class="mb-1 block font-medium">Name</span><TextInput
							tone="admin"
							bind:value={seoJSONLDName}
						/></label
					>{/if}
			</div>
		</div>
		<div class="mt-6 flex justify-end">
			<Button tone="admin" variant="primary" disabled={seoSaving} onclick={() => void savePageSEO()}
				><i class="bi bi-floppy mr-1"></i>{seoSaving ? "Saving..." : "Save SEO"}</Button
			>
		</div>
	{/if}
</AdminPanel>
