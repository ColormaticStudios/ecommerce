<script lang="ts">
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import { adminSurfaceVariantClasses } from "$lib/admin/tokens";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import type { EditorLayout } from "./types";

	let {
		layout,
		title = $bindable(),
		description = $bindable(),
		canonicalPath = $bindable(),
		ogImageMediaId = $bindable(),
		noIndex = $bindable(),
	}: {
		layout: EditorLayout;
		title: string;
		description: string;
		canonicalPath: string;
		ogImageMediaId: string;
		noIndex: boolean;
	} = $props();
</script>

<div class={layout === "split" ? adminSurfaceVariantClasses["panel-tight"] : ""}>
	<AdminFieldLabel>SEO</AdminFieldLabel>
	<div class="mt-4 grid gap-4 sm:grid-cols-2">
		<div>
			<AdminFieldLabel>SEO title</AdminFieldLabel><TextInput
				tone="admin"
				class="mt-1"
				type="text"
				aria-label="SEO title"
				bind:value={title}
			/>
		</div>
		<div>
			<AdminFieldLabel>Canonical path</AdminFieldLabel><TextInput
				tone="admin"
				class="mt-1"
				type="text"
				aria-label="Canonical path"
				bind:value={canonicalPath}
			/>
		</div>
		<div class="sm:col-span-2">
			<AdminFieldLabel>SEO description</AdminFieldLabel><TextArea
				tone="admin"
				class="mt-1"
				rows={3}
				aria-label="SEO description"
				bind:value={description}
			/>
		</div>
		<div>
			<AdminFieldLabel>OG image media ID</AdminFieldLabel><TextInput
				tone="admin"
				class="mt-1"
				type="text"
				aria-label="OG image media ID"
				bind:value={ogImageMediaId}
			/>
		</div>
		<label class="mt-6 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200"
			><input type="checkbox" bind:checked={noIndex} />Prevent indexing</label
		>
	</div>
</div>
