<script lang="ts">
	import type { CmsContentBlock } from "$lib/cms";
	import Button from "$lib/components/Button.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	type EditableBlock = CmsContentBlock & { editorId: string };
	type FooterBlock = Extract<EditableBlock, { type: "footer" }>;
	type PromoBlock = Extract<EditableBlock, { type: "promo_banner" }>;
	type Region = "announcement_bar" | "sitewide_banner" | "trust_strip" | "footer";

	interface Props {
		region: Region;
		blocks: EditableBlock[];
	}

	let { region, blocks = $bindable() }: Props = $props();
	let idSequence = 0;

	const promo = $derived(
		blocks.find((block): block is PromoBlock => block.type === "promo_banner")
	);
	const trustItems = $derived(
		blocks.filter(
			(block): block is Extract<EditableBlock, { type: "rich_text" }> => block.type === "rich_text"
		)
	);
	const footer = $derived(blocks.find((block): block is FooterBlock => block.type === "footer"));

	function id(prefix: string) {
		idSequence += 1;
		return `${prefix}-${Date.now()}-${idSequence}`;
	}

	function updatePromo(updates: Partial<PromoBlock>) {
		if (!promo) return;
		blocks = blocks.map((block) =>
			block.editorId === promo.editorId ? ({ ...block, ...updates } as EditableBlock) : block
		);
	}

	function updateTrustItem(editorId: string, body: string) {
		blocks = blocks.map((block) => (block.editorId === editorId ? { ...block, body } : block));
	}

	function addTrustItem() {
		blocks = [...blocks, { editorId: id("trust"), type: "rich_text", body: "New benefit" }];
	}

	function removeTrustItem(editorId: string) {
		blocks = blocks.filter((block) => block.editorId !== editorId);
	}

	function updateFooter(updates: Partial<FooterBlock>) {
		if (!footer) return;
		blocks = blocks.map((block) =>
			block.editorId === footer.editorId ? ({ ...block, ...updates } as EditableBlock) : block
		);
	}

	function updateColumn(index: number, updates: Partial<FooterBlock["columns"][number]>) {
		if (!footer) return;
		const columns = footer.columns.map((column, columnIndex) =>
			columnIndex === index ? { ...column, ...updates } : column
		);
		updateFooter({ columns });
	}

	function addColumn() {
		if (!footer || footer.columns.length >= 6) return;
		updateFooter({ columns: [...footer.columns, { title: "New column", links: [] }] });
	}

	function removeColumn(index: number) {
		if (!footer) return;
		updateFooter({ columns: footer.columns.filter((_, columnIndex) => columnIndex !== index) });
	}

	function addColumnLink(columnIndex: number) {
		if (!footer) return;
		const column = footer.columns[columnIndex];
		if (!column || column.links.length >= 10) return;
		updateColumn(columnIndex, { links: [...column.links, { label: "New link", url: "/" }] });
	}

	function updateColumnLink(
		columnIndex: number,
		linkIndex: number,
		key: "label" | "url",
		value: string
	) {
		if (!footer) return;
		const column = footer.columns[columnIndex];
		if (!column) return;
		updateColumn(columnIndex, {
			links: column.links.map((link, index) =>
				index === linkIndex ? { ...link, [key]: value } : link
			),
		});
	}

	function removeColumnLink(columnIndex: number, linkIndex: number) {
		if (!footer) return;
		const column = footer.columns[columnIndex];
		if (!column) return;
		updateColumn(columnIndex, { links: column.links.filter((_, index) => index !== linkIndex) });
	}

	function addSocialLink() {
		if (!footer || footer.social_links.length >= 8) return;
		updateFooter({
			social_links: [...footer.social_links, { label: "Social link", url: "https://" }],
		});
	}

	function updateSocialLink(index: number, key: "label" | "url", value: string) {
		if (!footer) return;
		updateFooter({
			social_links: footer.social_links.map((link, linkIndex) =>
				linkIndex === index ? { ...link, [key]: value } : link
			),
		});
	}

	function removeSocialLink(index: number) {
		if (!footer) return;
		updateFooter({
			social_links: footer.social_links.filter((_, linkIndex) => linkIndex !== index),
		});
	}

	const footerPreviewClass =
		"bg-white text-stone-900 dark:border dark:border-stone-800 dark:bg-stone-950 dark:text-stone-100";
</script>

{#if region === "announcement_bar" || region === "sitewide_banner"}
	{#if promo}
		<div class="space-y-5">
			<div class="rounded-md bg-stone-950 px-5 py-4 text-center text-sm text-white">
				<strong>{promo.title || "Announcement"}</strong>
				{#if promo.body}<span class="ml-2 text-stone-300">{promo.body}</span>{/if}
				{#if promo.link?.label}<span class="ml-3 font-semibold underline">{promo.link.label}</span
					>{/if}
			</div>
			<div class="grid gap-4 md:grid-cols-2">
				<label class="text-sm"
					><span class="mb-1 block font-medium">Headline</span><TextInput
						tone="admin"
						value={promo.title}
						oninput={(event) => updatePromo({ title: event.currentTarget.value })}
					/></label
				>
				<label class="text-sm"
					><span class="mb-1 block font-medium">Message</span><TextInput
						tone="admin"
						value={promo.body ?? ""}
						oninput={(event) => updatePromo({ body: event.currentTarget.value })}
					/></label
				>
				<label class="text-sm"
					><span class="mb-1 block font-medium">Link label</span><TextInput
						tone="admin"
						value={promo.link?.label ?? ""}
						oninput={(event) =>
							updatePromo({
								link: { label: event.currentTarget.value, url: promo.link?.url ?? "" },
							})}
					/></label
				>
				<label class="text-sm"
					><span class="mb-1 block font-medium">Link destination</span><TextInput
						tone="admin"
						value={promo.link?.url ?? ""}
						oninput={(event) =>
							updatePromo({
								link: { label: promo.link?.label ?? "", url: event.currentTarget.value },
							})}
					/></label
				>
			</div>
		</div>
	{/if}
{:else if region === "trust_strip"}
	<div class="space-y-5">
		<div
			class="flex flex-wrap justify-center gap-x-8 gap-y-2 border-y border-stone-200 bg-stone-50 px-4 py-4 text-sm font-medium dark:border-stone-800 dark:bg-stone-900"
		>
			{#each trustItems as item (item.editorId)}<span
					><i class="bi bi-check-circle mr-1.5"></i>{item.body}</span
				>{/each}
		</div>
		<div
			class="divide-y divide-stone-200 border-y border-stone-200 dark:divide-stone-800 dark:border-stone-800"
		>
			{#each trustItems as item, index (item.editorId)}
				<div class="flex items-center gap-2 py-3">
					<TextInput
						tone="admin"
						value={item.body}
						aria-label={`Trust item ${index + 1}`}
						oninput={(event) => updateTrustItem(item.editorId, event.currentTarget.value)}
					/>
					<IconButton
						tone="admin"
						variant="danger"
						outlined={true}
						size="sm"
						aria-label={`Remove trust item ${index + 1}`}
						title="Remove"
						onclick={() => removeTrustItem(item.editorId)}><i class="bi bi-trash"></i></IconButton
					>
				</div>
			{/each}
		</div>
		<Button tone="admin" size="small" onclick={addTrustItem}
			><i class="bi bi-plus-lg mr-1"></i>Add benefit</Button
		>
	</div>
{:else if footer}
	<div class="space-y-7">
		<div class={`rounded-md p-6 ${footerPreviewClass}`}>
			<div
				class={footer.layout === "centered"
					? "text-center"
					: footer.layout === "minimal"
						? "flex flex-wrap items-center justify-between gap-4"
						: "grid gap-8 md:grid-cols-[1.2fr_2fr]"}
			>
				<div>
					<div class="text-lg font-semibold">{footer.brand_name || "Brand"}</div>
					{#if footer.tagline}<p class="mt-2 max-w-sm text-sm opacity-70">{footer.tagline}</p>{/if}
				</div>
				{#if footer.layout !== "minimal"}<div
						class={`grid gap-6 ${footer.layout === "centered" ? "mt-6 sm:grid-cols-3" : "sm:grid-cols-2 lg:grid-cols-3"}`}
					>
						{#each footer.columns as column, columnIndex (columnIndex)}<div>
								<div class="text-sm font-semibold">{column.title}</div>
								<div class="mt-2 space-y-1 text-sm opacity-70">
									{#each column.links as link, linkIndex (linkIndex)}<div>{link.label}</div>{/each}
								</div>
							</div>{/each}
					</div>{/if}
			</div>
			<div
				class="mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-current/15 pt-4 text-xs opacity-70"
			>
				<span>{footer.copyright}</span><span
					>{footer.social_links.map((link) => link.label).join(" · ")}</span
				>
			</div>
		</div>

		<section class="space-y-4">
			<h3 class="text-sm font-semibold">Brand and layout</h3>
			<div class="grid gap-4 md:grid-cols-2">
				<label class="text-sm"
					><span class="mb-1 block font-medium">Brand name</span><TextInput
						tone="admin"
						value={footer.brand_name}
						oninput={(event) => updateFooter({ brand_name: event.currentTarget.value })}
					/></label
				>
				<label class="text-sm"
					><span class="mb-1 block font-medium">Copyright</span><TextInput
						tone="admin"
						value={footer.copyright}
						oninput={(event) => updateFooter({ copyright: event.currentTarget.value })}
					/></label
				>
				<label class="text-sm md:col-span-2"
					><span class="mb-1 block font-medium">Tagline</span><TextArea
						tone="admin"
						class="min-h-20"
						value={footer.tagline ?? ""}
						oninput={(event) => updateFooter({ tagline: event.currentTarget.value })}
					/></label
				>
			</div>
			<div>
				<div>
					<span class="mb-1 block text-sm font-medium">Layout</span>
					<div class="inline-flex rounded-md border border-stone-300 p-1 dark:border-stone-700">
						{#each [["columns", "Columns"], ["centered", "Centered"], ["minimal", "Minimal"]] as option (option[0])}<button
								type="button"
								class={`rounded px-3 py-1.5 text-xs font-medium ${footer.layout === option[0] ? "bg-stone-900 text-white dark:bg-stone-100 dark:text-stone-900" : "text-stone-600 dark:text-stone-300"}`}
								onclick={() => updateFooter({ layout: option[0] as FooterBlock["layout"] })}
								>{option[1]}</button
							>{/each}
					</div>
				</div>
			</div>
		</section>

		<section>
			<div class="mb-2 flex items-center justify-between gap-3">
				<h3 class="text-sm font-semibold">Link columns</h3>
				<Button tone="admin" size="small" onclick={addColumn} disabled={footer.columns.length >= 6}
					><i class="bi bi-plus-lg mr-1"></i>Add column</Button
				>
			</div>
			<div
				class="divide-y divide-stone-200 border-y border-stone-200 dark:divide-stone-800 dark:border-stone-800"
			>
				{#each footer.columns as column, columnIndex (columnIndex)}
					<div class="space-y-3 py-4">
						<div class="flex items-center gap-2">
							<TextInput
								tone="admin"
								value={column.title}
								aria-label={`Column ${columnIndex + 1} title`}
								oninput={(event) => updateColumn(columnIndex, { title: event.currentTarget.value })}
							/><IconButton
								tone="admin"
								variant="danger"
								outlined={true}
								size="sm"
								aria-label={`Remove column ${columnIndex + 1}`}
								title="Remove column"
								onclick={() => removeColumn(columnIndex)}><i class="bi bi-trash"></i></IconButton
							>
						</div>
						<div class="space-y-2 pl-4">
							{#each column.links as link, linkIndex (linkIndex)}<div
									class="grid grid-cols-[1fr_1.3fr_auto] gap-2"
								>
									<TextInput
										tone="admin"
										value={link.label}
										aria-label={`Column ${columnIndex + 1} link ${linkIndex + 1} label`}
										oninput={(event) =>
											updateColumnLink(columnIndex, linkIndex, "label", event.currentTarget.value)}
									/><TextInput
										tone="admin"
										value={link.url}
										aria-label={`Column ${columnIndex + 1} link ${linkIndex + 1} destination`}
										oninput={(event) =>
											updateColumnLink(columnIndex, linkIndex, "url", event.currentTarget.value)}
									/><IconButton
										tone="admin"
										variant="danger"
										outlined={true}
										size="sm"
										aria-label="Remove link"
										title="Remove link"
										onclick={() => removeColumnLink(columnIndex, linkIndex)}
										><i class="bi bi-x-lg"></i></IconButton
									>
								</div>{/each}<Button
								tone="admin"
								size="small"
								onclick={() => addColumnLink(columnIndex)}
								disabled={column.links.length >= 10}>Add link</Button
							>
						</div>
					</div>
				{/each}
			</div>
		</section>

		<section>
			<div class="mb-2 flex items-center justify-between gap-3">
				<h3 class="text-sm font-semibold">Social links</h3>
				<Button
					tone="admin"
					size="small"
					onclick={addSocialLink}
					disabled={footer.social_links.length >= 8}
					><i class="bi bi-plus-lg mr-1"></i>Add social link</Button
				>
			</div>
			<div class="space-y-2">
				{#each footer.social_links as link, index (index)}<div
						class="grid grid-cols-[1fr_1.3fr_auto] gap-2"
					>
						<TextInput
							tone="admin"
							value={link.label}
							aria-label={`Social link ${index + 1} label`}
							oninput={(event) => updateSocialLink(index, "label", event.currentTarget.value)}
						/><TextInput
							tone="admin"
							value={link.url}
							aria-label={`Social link ${index + 1} destination`}
							oninput={(event) => updateSocialLink(index, "url", event.currentTarget.value)}
						/><IconButton
							tone="admin"
							variant="danger"
							outlined={true}
							size="sm"
							aria-label={`Remove social link ${index + 1}`}
							title="Remove"
							onclick={() => removeSocialLink(index)}><i class="bi bi-x-lg"></i></IconButton
						>
					</div>{/each}
			</div>
		</section>
	</div>
{/if}
