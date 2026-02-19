<script lang="ts">
	import { getContext, onDestroy, onMount } from "svelte";
	import { type API } from "$lib/api";
	import Alert from "$lib/components/Alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import ButtonInput from "$lib/components/ButtonInput.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import {
		type StorefrontFooterColumnModel,
		type StorefrontHomepageSectionModel,
		type StorefrontLinkModel,
		type StorefrontSettingsModel,
		cloneStorefrontSettings,
		createDefaultHeroSection,
		createDefaultProductSection,
		createDefaultPromoCard,
		createDefaultStorefrontSettings,
		STOREFRONT_LIMITS,
	} from "$lib/storefront";

	interface Props {
		showInlineMessages?: boolean;
		showInlineUnsavedNotice?: boolean;
		onErrorMessage?: (message: string) => void;
		onStatusMessage?: (message: string) => void;
		onDirtyChange?: (dirty: boolean) => void;
		onSaveRequestChange?: (saveAction: (() => Promise<void>) | null) => void;
	}

	let {
		showInlineMessages = true,
		showInlineUnsavedNotice = true,
		onErrorMessage,
		onStatusMessage,
		onDirtyChange,
		onSaveRequestChange,
	}: Props = $props();

	const api: API = getContext("api");

	const maxSectionPromoCards = STOREFRONT_LIMITS.max_section_promo_cards;
	const maxFooterColumns = STOREFRONT_LIMITS.max_footer_columns;
	const maxFooterLinksPerColumn = STOREFRONT_LIMITS.max_footer_links_per_column;
	const maxSocialLinks = STOREFRONT_LIMITS.max_social_links;
	const maxHomepageSections = STOREFRONT_LIMITS.max_homepage_sections;
	const maxSectionBadges = STOREFRONT_LIMITS.max_section_badges;

	let loading = $state(true);
	let saving = $state(false);
	let uploadingHero = $state(false);
	let errorMessage = $state("");
	let statusMessage = $state("");
	let lastUpdated = $state<Date | null>(null);
	let heroPreviewUrls = $state<Record<string, string>>({});
	let savedSnapshot = $state("");
	let newSectionType = $state<StorefrontHomepageSectionModel["type"]>("products");

	const initialSettings = createDefaultStorefrontSettings();
	let draft = $state<StorefrontSettingsModel>(cloneStorefrontSettings(initialSettings));
	let manualProductIdsInputBySection = $state<Record<string, string>>({});
	const currentSnapshot = $derived(JSON.stringify(buildPersistedStorefrontPayload(draft)));
	const hasUnsavedChanges = $derived(!loading && currentSnapshot !== savedSnapshot);

	$effect(() => {
		onDirtyChange?.(hasUnsavedChanges);
	});

	$effect(() => {
		onSaveRequestChange?.(hasUnsavedChanges ? saveStorefrontSettings : null);
	});

	function createEmptyLink(): StorefrontLinkModel {
		return { label: "", url: "" };
	}

	function createEmptyFooterColumn(): StorefrontFooterColumnModel {
		return { title: "", links: [createEmptyLink()] };
	}

	function createSection(
		type: StorefrontHomepageSectionModel["type"] = "products"
	): StorefrontHomepageSectionModel {
		const section: StorefrontHomepageSectionModel = {
			id: `${type}-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
			type,
			enabled: true,
		};

		if (type === "products") {
			section.product_section = createDefaultProductSection();
		}
		if (type === "hero") {
			section.hero = createDefaultHeroSection();
		}
		if (type === "promo_cards") {
			section.promo_card_limit = 1;
			section.promo_cards = [createDefaultPromoCard()];
		}
		if (type === "badges") {
			section.badges = [""];
		}

		return section;
	}

	function syncManualProductInputFromDraft() {
		const next: Record<string, string> = {};
		for (const section of draft.homepage_sections) {
			if (section.type !== "products" || !section.product_section) {
				continue;
			}
			next[section.id] = section.product_section.product_ids.join(", ");
		}
		manualProductIdsInputBySection = next;
	}

	function ensurePromoCardShape(section: StorefrontHomepageSectionModel) {
		if (section.type !== "promo_cards") {
			return;
		}
		const limit = Math.min(
			maxSectionPromoCards,
			Math.max(1, Number(section.promo_card_limit ?? 1))
		);
		section.promo_card_limit = limit;
		const cards = (section.promo_cards ?? []).slice(0, maxSectionPromoCards);
		while (cards.length < limit) {
			cards.push(createDefaultPromoCard());
		}
		section.promo_cards = cards;
	}

	function ensureBadgeShape(section: StorefrontHomepageSectionModel) {
		if (section.type !== "badges") {
			return;
		}
		const badges = (section.badges ?? []).slice(0, maxSectionBadges);
		section.badges = badges.length > 0 ? badges : [""];
	}

	function ensureSettingsShape(settings: StorefrontSettingsModel) {
		if (settings.site_title.trim() === "") {
			settings.site_title = "Ecommerce";
		}

		if (settings.homepage_sections.length > maxHomepageSections) {
			settings.homepage_sections = settings.homepage_sections.slice(0, maxHomepageSections);
		}
		if (settings.homepage_sections.length === 0) {
			settings.homepage_sections = [
				{ id: "hero", type: "hero", enabled: true },
				createSection("products"),
			];
		}

		settings.homepage_sections = settings.homepage_sections.map((section, index) => {
			const id = section.id.trim() || `${section.type}-${index + 1}`;
			const normalized: StorefrontHomepageSectionModel = {
				...section,
				id,
			};
			if (section.type === "products") {
				normalized.product_section = section.product_section ?? createDefaultProductSection();
			}
			if (section.type === "hero") {
				normalized.hero = section.hero ?? createDefaultHeroSection();
			}
			if (section.type === "promo_cards") {
				normalized.promo_cards = section.promo_cards ?? [createDefaultPromoCard()];
				normalized.promo_card_limit = section.promo_card_limit ?? 1;
				ensurePromoCardShape(normalized);
			}
			if (section.type === "badges") {
				normalized.badges = section.badges ?? [""];
				ensureBadgeShape(normalized);
			}
			if (section.type !== "hero") {
				normalized.hero = undefined;
			}
			if (section.type !== "products") {
				normalized.product_section = undefined;
			}
			return normalized;
		});

		if (settings.footer.columns.length === 0) {
			settings.footer.columns = [createEmptyFooterColumn()];
		}
		if (settings.footer.columns.length > maxFooterColumns) {
			settings.footer.columns = settings.footer.columns.slice(0, maxFooterColumns);
		}
		settings.footer.columns = settings.footer.columns.map((column) => {
			const links = column.links.length > 0 ? column.links : [createEmptyLink()];
			return {
				...column,
				links: links.slice(0, maxFooterLinksPerColumn),
			};
		});

		settings.footer.social_links = settings.footer.social_links.slice(0, maxSocialLinks);
	}

	function ensureDraftShape() {
		ensureSettingsShape(draft);
	}

	function buildPersistedStorefrontPayload(
		source: StorefrontSettingsModel
	): StorefrontSettingsModel {
		const payload = cloneStorefrontSettings(source);
		ensureSettingsShape(payload);
		payload.footer.columns = payload.footer.columns.map((column) => ({
			...column,
			links: column.links.filter((link) => link.label.trim() !== "" || link.url.trim() !== ""),
		}));
		payload.footer.social_links = payload.footer.social_links.filter(
			(link) => link.label.trim() !== "" || link.url.trim() !== ""
		);
		payload.homepage_sections = payload.homepage_sections.map((section) => {
			if (section.type === "badges") {
				return {
					...section,
					badges: (section.badges ?? []).map((badge) => badge.trim()).filter(Boolean),
				};
			}
			return section;
		});
		return payload;
	}

	function hydrateDraft(source: StorefrontSettingsModel) {
		draft = cloneStorefrontSettings(source);
		ensureDraftShape();
		syncManualProductInputFromDraft();
		savedSnapshot = JSON.stringify(buildPersistedStorefrontPayload(draft));
	}

	function parseProductIDs(value: string): number[] {
		const seen: number[] = [];
		return value
			.split(",")
			.map((id) => Number(id.trim()))
			.filter((id) => Number.isInteger(id) && id > 0)
			.filter((id) => {
				if (seen.includes(id)) {
					return false;
				}
				seen.push(id);
				return true;
			});
	}

	function setManualProductIDsInput(section: StorefrontHomepageSectionModel, value: string) {
		if (section.type !== "products" || !section.product_section) {
			return;
		}
		manualProductIdsInputBySection[section.id] = value;
		section.product_section.product_ids = parseProductIDs(value);
	}

	function moveSection(sectionIndex: number, direction: -1 | 1) {
		const nextIndex = sectionIndex + direction;
		if (nextIndex < 0 || nextIndex >= draft.homepage_sections.length) {
			return;
		}
		const next = [...draft.homepage_sections];
		const current = next[sectionIndex];
		next[sectionIndex] = next[nextIndex];
		next[nextIndex] = current;
		draft.homepage_sections = next;
	}

	function clearHeroPreview(sectionID: string) {
		const preview = heroPreviewUrls[sectionID];
		if (!preview) {
			return;
		}
		URL.revokeObjectURL(preview);
		delete heroPreviewUrls[sectionID];
		heroPreviewUrls = { ...heroPreviewUrls };
	}

	function clearAllHeroPreviews() {
		Object.keys(heroPreviewUrls).forEach((sectionID) => clearHeroPreview(sectionID));
	}

	async function handleHeroImageUpload(section: StorefrontHomepageSectionModel, event: Event) {
		if (section.type !== "hero") {
			return;
		}
		const target = event.target as HTMLInputElement;
		const file = target.files?.[0];
		target.value = "";
		if (!file) {
			return;
		}
		if (!file.type.startsWith("image/")) {
			errorMessage = "Hero background must be an image file.";
			return;
		}

		uploadingHero = true;
		errorMessage = "";
		statusMessage = "";
		try {
			const mediaID = await api.uploadMedia(file);
			section.hero = section.hero ?? createDefaultHeroSection();
			section.hero.background_image_media_id = mediaID;
			clearHeroPreview(section.id);
			heroPreviewUrls = { ...heroPreviewUrls, [section.id]: URL.createObjectURL(file) };
			statusMessage = "Hero background uploaded. Save storefront to publish this image.";
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to upload hero background image.";
		} finally {
			uploadingHero = false;
		}
	}

	function removeHeroImage(section: StorefrontHomepageSectionModel) {
		if (section.type !== "hero") {
			return;
		}
		section.hero = section.hero ?? createDefaultHeroSection();
		clearHeroPreview(section.id);
		section.hero.background_image_media_id = "";
		section.hero.background_image_url = "";
	}

	function collectMissingFields(settings: StorefrontSettingsModel): string[] {
		const missing: string[] = [];
		const addMissing = (label: string) => {
			if (!missing.includes(label)) {
				missing.push(label);
			}
		};
		const addIfBlank = (label: string, value: string) => {
			if (value.trim() === "") {
				addMissing(label);
			}
		};
		const addIfLinkPartial = (label: string, link: StorefrontLinkModel) => {
			const hasLabel = link.label.trim() !== "";
			const hasUrl = link.url.trim() !== "";
			if (hasLabel !== hasUrl) {
				addMissing(label);
			}
		};

		addIfBlank("Site title", settings.site_title);
		addIfBlank("Footer brand name", settings.footer.brand_name);
		addIfBlank("Footer copyright", settings.footer.copyright);

		settings.homepage_sections.forEach((section, sectionIndex) => {
			const sectionLabel = `Section ${sectionIndex + 1}`;
			if (section.id.trim() === "") {
				addMissing(`${sectionLabel} ID`);
			}
			if (section.type === "hero" && section.hero) {
				addIfBlank(`${sectionLabel} hero title`, section.hero.title);
				addIfBlank(`${sectionLabel} hero subtitle`, section.hero.subtitle);
				addIfLinkPartial(
					`${sectionLabel} hero primary CTA (label and URL)`,
					section.hero.primary_cta
				);
				addIfLinkPartial(
					`${sectionLabel} hero secondary CTA (label and URL)`,
					section.hero.secondary_cta
				);
			}
			if (section.type === "products" && section.product_section) {
				addIfBlank(`${sectionLabel} product title`, section.product_section.title);
				if (section.product_section.source === "search") {
					addIfBlank(`${sectionLabel} search query`, section.product_section.query);
				}
			}
			if (section.type === "promo_cards") {
				const cards = (section.promo_cards ?? []).slice(0, section.promo_card_limit ?? 1);
				if (cards.length === 0) {
					addMissing(`${sectionLabel} promo cards`);
				}
				cards.forEach((card, cardIndex) => {
					const cardLabel = `${sectionLabel} promo card ${cardIndex + 1}`;
					addIfBlank(`${cardLabel} title`, card.title);
					addIfBlank(`${cardLabel} description`, card.description);
					addIfLinkPartial(`${cardLabel} link (label and URL)`, card.link);
				});
			}
			if (section.type === "badges") {
				const nonEmpty = (section.badges ?? []).map((value) => value.trim()).filter(Boolean);
				if (nonEmpty.length === 0) {
					addMissing(`${sectionLabel} badges`);
				}
			}
		});

		settings.footer.columns.forEach((column, columnIndex) => {
			addIfBlank(`Footer column ${columnIndex + 1} title`, column.title);
			column.links.forEach((link, linkIndex) => {
				if (link.label.trim() === "" && link.url.trim() === "") {
					return;
				}
				addIfLinkPartial(
					`Footer column ${columnIndex + 1} link ${linkIndex + 1} (label and URL)`,
					link
				);
			});
		});

		settings.footer.social_links.forEach((link, linkIndex) => {
			if (link.label.trim() === "" && link.url.trim() === "") {
				return;
			}
			addIfLinkPartial(`Social link ${linkIndex + 1} (label and URL)`, link);
		});

		return missing;
	}

	function getReadableSaveError(err: unknown): string {
		if (typeof err === "object" && err !== null) {
			const withBody = err as {
				body?: unknown;
				message?: unknown;
				status?: unknown;
				statusText?: unknown;
			};
			if (typeof withBody.body === "object" && withBody.body !== null && "error" in withBody.body) {
				const bodyError = (withBody.body as { error?: unknown }).error;
				if (typeof bodyError === "string" && bodyError.trim() !== "") {
					return bodyError;
				}
			}
			if (typeof withBody.body === "string" && withBody.body.trim() !== "") {
				return withBody.body;
			}
			if (typeof withBody.message === "string" && withBody.message.trim() !== "") {
				return withBody.message;
			}
			if (typeof withBody.status === "number" && typeof withBody.statusText === "string") {
				return `${withBody.status} ${withBody.statusText}`;
			}
		}
		if (typeof err === "string" && err.trim() !== "") {
			return err;
		}
		return "Unknown error";
	}

	async function loadStorefrontSettings() {
		loading = true;
		errorMessage = "";
		statusMessage = "";
		onErrorMessage?.("");
		onStatusMessage?.("");
		try {
			const response = await api.getAdminStorefrontSettings();
			lastUpdated = response.updated_at;
			clearAllHeroPreviews();
			hydrateDraft(response.settings);
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load storefront settings.";
			onErrorMessage?.(errorMessage);
			lastUpdated = null;
			clearAllHeroPreviews();
			hydrateDraft(createDefaultStorefrontSettings());
		} finally {
			loading = false;
		}
	}

	async function saveStorefrontSettings() {
		saving = true;
		errorMessage = "";
		statusMessage = "";
		onErrorMessage?.("");
		onStatusMessage?.("");
		try {
			ensureDraftShape();
			const payload = buildPersistedStorefrontPayload(draft);

			const missingFields = collectMissingFields(payload);
			if (missingFields.length > 0) {
				errorMessage = `Please fill the required fields before saving: ${missingFields.join("; ")}.`;
				onErrorMessage?.(errorMessage);
				return;
			}

			const response = await api.updateStorefrontSettings(payload);
			lastUpdated = response.updated_at;
			clearAllHeroPreviews();
			hydrateDraft(response.settings);
			statusMessage = "Storefront settings saved.";
			onStatusMessage?.(statusMessage);
		} catch (err) {
			console.error(err);
			errorMessage = `Unable to save storefront settings: ${getReadableSaveError(err)}.`;
			onErrorMessage?.(errorMessage);
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		void loadStorefrontSettings();
	});

	onDestroy(() => {
		clearAllHeroPreviews();
		onDirtyChange?.(false);
		onSaveRequestChange?.(null);
	});
</script>

<div
	class="mt-6 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
>
	<div class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Storefront</h2>
			{#if lastUpdated}
				<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
					Last saved {lastUpdated.toLocaleString()}
				</p>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			<Button
				variant="regular"
				type="button"
				onclick={loadStorefrontSettings}
				disabled={loading || saving || uploadingHero}
			>
				<i class="bi bi-arrow-clockwise mr-1"></i>
				Refresh
			</Button>
			<Button
				variant="primary"
				type="button"
				onclick={saveStorefrontSettings}
				disabled={loading || saving || uploadingHero}
			>
				<i class="bi bi-floppy-fill mr-1"></i>
				{saving ? "Saving..." : "Save storefront"}
			</Button>
		</div>
	</div>

	{#if showInlineMessages && errorMessage}
		<div class="mt-5">
			<Alert
				message={errorMessage}
				tone="error"
				icon="bi-x-circle-fill"
				onClose={() => (errorMessage = "")}
			/>
		</div>
	{/if}
	{#if showInlineMessages && statusMessage}
		<div class="mt-5">
			<Alert
				message={statusMessage}
				tone="success"
				icon="bi-check-circle-fill"
				onClose={() => (statusMessage = "")}
			/>
		</div>
	{/if}

	{#if loading}
		<div
			class="mt-6 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-600 dark:border-gray-800 dark:bg-gray-950 dark:text-gray-300"
		>
			Loading storefront settings...
		</div>
	{:else}
		<div class="mt-6 space-y-6">
			<section class="rounded-xl border border-gray-200 p-4 dark:border-gray-800">
				<h3
					class="text-sm font-semibold tracking-[0.18em] text-gray-500 uppercase dark:text-gray-400"
				>
					Site title
				</h3>
				<div class="mt-4">
					<TextInput placeholder="Navbar site title" bind:value={draft.site_title} />
				</div>
			</section>

			<section class="rounded-xl border border-gray-200 p-4 dark:border-gray-800">
				<h3
					class="text-sm font-semibold tracking-[0.18em] text-gray-500 uppercase dark:text-gray-400"
				>
					Homepage layout
				</h3>

				<div class="mt-4 space-y-4">
					{#each draft.homepage_sections as section, sectionIndex (section.id)}
						<div class="rounded-lg border border-gray-200 p-4 dark:border-gray-800">
							<div class="flex flex-wrap items-center gap-3">
								<label class="flex items-center gap-2 text-sm">
									<input type="checkbox" bind:checked={section.enabled} />
									Enabled
								</label>
								<TextInput
									class="min-w-48 flex-1"
									placeholder="Section ID"
									bind:value={section.id}
								/>
								<select
									class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
									bind:value={section.type}
									onchange={() => {
										if (section.type !== "hero") {
											clearHeroPreview(section.id);
										}
										section.hero =
											section.type === "hero"
												? (section.hero ?? createDefaultHeroSection())
												: undefined;
										section.product_section =
											section.type === "products"
												? (section.product_section ?? createDefaultProductSection())
												: undefined;
										if (section.type === "promo_cards") {
											section.promo_card_limit = section.promo_card_limit ?? 1;
											section.promo_cards = section.promo_cards ?? [createDefaultPromoCard()];
											ensurePromoCardShape(section);
										} else {
											section.promo_card_limit = undefined;
											section.promo_cards = undefined;
										}
										if (section.type === "badges") {
											section.badges = section.badges ?? [""];
											ensureBadgeShape(section);
										} else {
											section.badges = undefined;
										}
										draft = cloneStorefrontSettings(draft);
										syncManualProductInputFromDraft();
									}}
								>
									<option value="hero">Hero</option>
									<option value="products">Products</option>
									<option value="promo_cards">Promo cards</option>
									<option value="badges">Badges</option>
								</select>
								<div class="ml-auto flex items-center gap-1">
									<IconButton
										variant="neutral"
										type="button"
										onclick={() => moveSection(sectionIndex, -1)}
										aria-label="Move section up"
									>
										<i class="bi bi-arrow-up"></i>
									</IconButton>
									<IconButton
										variant="neutral"
										type="button"
										onclick={() => moveSection(sectionIndex, 1)}
										aria-label="Move section down"
									>
										<i class="bi bi-arrow-down"></i>
									</IconButton>
									<IconButton
										variant="danger"
										type="button"
										disabled={draft.homepage_sections.length <= 1}
										onclick={() => {
											if (draft.homepage_sections.length <= 1) {
												return;
											}
											draft.homepage_sections = draft.homepage_sections.filter(
												(_, idx) => idx !== sectionIndex
											);
											syncManualProductInputFromDraft();
										}}
										aria-label="Remove section"
									>
										<i class="bi bi-trash"></i>
									</IconButton>
								</div>
							</div>

							{#if section.type === "products" && section.product_section}
								<div class="mt-4 grid gap-3">
									<div class="grid gap-3 md:grid-cols-2">
										<TextInput
											placeholder="Section title"
											bind:value={section.product_section.title}
										/>
										<TextInput
											placeholder="Section subtitle"
											bind:value={section.product_section.subtitle}
										/>
									</div>
									<div class="grid gap-3 md:grid-cols-4">
										<select
											class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
											bind:value={section.product_section.source}
										>
											<option value="newest">Source: newest</option>
											<option value="manual">Source: manual IDs</option>
											<option value="search">Source: search query</option>
										</select>
										<select
											class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
											bind:value={section.product_section.sort}
										>
											<option value="created_at">Sort: created at</option>
											<option value="price">Sort: price</option>
											<option value="name">Sort: name</option>
										</select>
										<select
											class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
											bind:value={section.product_section.order}
										>
											<option value="desc">Order: descending</option>
											<option value="asc">Order: ascending</option>
										</select>
										<NumberInput
											min={1}
											max={STOREFRONT_LIMITS.max_product_section_limit}
											placeholder="Limit"
											bind:value={section.product_section.limit}
										/>
									</div>

									{#if section.product_section.source === "search"}
										<TextInput
											placeholder="Search query"
											bind:value={section.product_section.query}
										/>
									{/if}
									{#if section.product_section.source === "manual"}
										<TextInput
											placeholder="Manual product IDs (comma-separated)"
											value={manualProductIdsInputBySection[section.id] ??
												section.product_section.product_ids.join(", ")}
											oninput={(event) => {
												const target = event.target as HTMLInputElement;
												setManualProductIDsInput(section, target.value);
											}}
										/>
									{/if}

									<div class="grid gap-3 md:grid-cols-3">
										<label class="flex items-center gap-2 text-sm">
											<input
												type="checkbox"
												bind:checked={section.product_section.show_description}
											/>
											Show description
										</label>
										<label class="flex items-center gap-2 text-sm">
											<input type="checkbox" bind:checked={section.product_section.show_stock} />
											Show stock
										</label>
										<select
											class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
											bind:value={section.product_section.image_aspect}
										>
											<option value="square">Image: square</option>
											<option value="wide">Image: wide</option>
										</select>
									</div>
								</div>
							{/if}

							{#if section.type === "hero" && section.hero}
								{@const hero = section.hero}
								<div class="mt-4 grid gap-3">
									<TextInput placeholder="Eyebrow" bind:value={hero.eyebrow} />
									<TextInput placeholder="Hero title" bind:value={hero.title} />
									<textarea
										class="min-h-22 rounded-md border border-gray-300 bg-gray-200 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
										placeholder="Hero subtitle"
										bind:value={hero.subtitle}
									></textarea>
									<div class="rounded-lg border border-gray-200 p-3 dark:border-gray-800">
										<div class="flex flex-wrap items-center justify-between gap-3">
											<p class="text-sm text-gray-600 dark:text-gray-300">Background image</p>
											<div class="flex items-center gap-2">
												<ButtonInput
													type="file"
													accept="image/*"
													onchange={(event) => handleHeroImageUpload(section, event)}
													disabled={uploadingHero || saving}
													variant="regular"
													size="small"
												>
													<i class="bi bi-upload mr-1"></i>
													{uploadingHero ? "Uploading..." : "Upload image"}
												</ButtonInput>
												<IconButton
													variant="danger"
													type="button"
													onclick={() => removeHeroImage(section)}
													aria-label="Remove hero image"
												>
													<i class="bi bi-trash"></i>
												</IconButton>
											</div>
										</div>
										{#if heroPreviewUrls[section.id] || hero.background_image_url}
											<img
												src={heroPreviewUrls[section.id] || hero.background_image_url}
												alt="Hero background preview"
												class="mt-3 h-36 w-full rounded-md object-cover"
											/>
										{/if}
									</div>
									<div class="grid gap-3 md:grid-cols-2">
										<div class="grid grid-cols-2 gap-2">
											<TextInput
												placeholder="Primary CTA label"
												bind:value={hero.primary_cta.label}
											/>
											<TextInput placeholder="Primary CTA URL" bind:value={hero.primary_cta.url} />
										</div>
										<div class="grid grid-cols-2 gap-2">
											<TextInput
												placeholder="Secondary CTA label"
												bind:value={hero.secondary_cta.label}
											/>
											<TextInput
												placeholder="Secondary CTA URL"
												bind:value={hero.secondary_cta.url}
											/>
										</div>
									</div>
								</div>
							{/if}

							{#if section.type === "promo_cards"}
								<div class="mt-4 space-y-3">
									<div class="grid gap-3 md:grid-cols-[1fr_180px] md:items-center">
										<p class="text-sm text-gray-600 dark:text-gray-300">
											Configure cards for this section.
										</p>
										<NumberInput
											min={1}
											max={maxSectionPromoCards}
											placeholder="Card count"
											bind:value={section.promo_card_limit}
											oninput={() => {
												ensurePromoCardShape(section);
												draft = cloneStorefrontSettings(draft);
											}}
										/>
									</div>
									<div class="grid gap-3 lg:grid-cols-2">
										{#each (section.promo_cards ?? []).slice(0, section.promo_card_limit ?? 1) as card, cardIndex (cardIndex)}
											<div class="rounded-lg border border-gray-200 p-3 dark:border-gray-800">
												<div class="mb-1 flex items-center justify-between">
													<p
														class="text-xs font-semibold tracking-[0.18em] text-gray-400 uppercase dark:text-gray-500"
													>
														Card {cardIndex + 1}
													</p>
													<IconButton
														variant="danger"
														type="button"
														disabled={(section.promo_cards?.length ?? 0) <= 1}
														onclick={() => {
															if ((section.promo_cards?.length ?? 0) <= 1) {
																return;
															}
															section.promo_cards = (section.promo_cards ?? []).filter(
																(_, idx) => idx !== cardIndex
															);
															section.promo_card_limit = Math.max(1, section.promo_cards.length);
															draft = cloneStorefrontSettings(draft);
														}}
														aria-label="Remove promo card"
													>
														<i class="bi bi-trash"></i>
													</IconButton>
												</div>
												<div class="grid gap-2 md:grid-cols-2">
													<TextInput placeholder="Kicker" bind:value={card.kicker} />
													<TextInput placeholder="Title" bind:value={card.title} />
													<textarea
														class="min-h-20 rounded-md border border-gray-300 bg-gray-200 px-3 py-2 md:col-span-2 dark:border-gray-700 dark:bg-gray-800"
														placeholder="Description"
														bind:value={card.description}
													></textarea>
													<TextInput placeholder="Image URL" bind:value={card.image_url} />
													<TextInput placeholder="Link label" bind:value={card.link.label} />
													<TextInput
														class="md:col-span-2"
														placeholder="Link URL"
														bind:value={card.link.url}
													/>
												</div>
											</div>
										{/each}
									</div>
									<div class="flex justify-end">
										<IconButton
											variant="primary"
											type="button"
											disabled={(section.promo_cards?.length ?? 0) >= maxSectionPromoCards}
											onclick={() => {
												if ((section.promo_cards?.length ?? 0) >= maxSectionPromoCards) {
													return;
												}
												section.promo_cards = [
													...(section.promo_cards ?? []),
													createDefaultPromoCard(),
												];
												section.promo_card_limit = section.promo_cards.length;
												draft = cloneStorefrontSettings(draft);
											}}
											aria-label="Add promo card"
										>
											<i class="bi bi-plus-lg"></i>
										</IconButton>
									</div>
								</div>
							{/if}

							{#if section.type === "badges"}
								<div class="mt-4 space-y-2">
									{#if section.badges}
										{#each section.badges as badge, badgeIndex (badgeIndex)}
											<div class="flex items-center gap-2">
												<TextInput
													class="flex-1"
													placeholder="Badge text"
													title={badge}
													bind:value={section.badges[badgeIndex]}
												/>
												<IconButton
													variant="danger"
													type="button"
													disabled={(section.badges?.length ?? 0) <= 1}
													onclick={() => {
														if ((section.badges?.length ?? 0) <= 1) {
															return;
														}
														section.badges = (section.badges ?? []).filter(
															(_, idx) => idx !== badgeIndex
														);
														draft = cloneStorefrontSettings(draft);
													}}
													aria-label="Remove badge"
												>
													<i class="bi bi-dash-lg"></i>
												</IconButton>
											</div>
										{/each}
									{/if}
									<div class="flex justify-end">
										<IconButton
											variant="primary"
											type="button"
											disabled={(section.badges?.length ?? 0) >= maxSectionBadges}
											onclick={() => {
												if ((section.badges?.length ?? 0) >= maxSectionBadges) {
													return;
												}
												section.badges = [...(section.badges ?? []), ""];
												draft = cloneStorefrontSettings(draft);
											}}
											aria-label="Add badge"
										>
											<i class="bi bi-plus-lg"></i>
										</IconButton>
									</div>
								</div>
							{/if}
						</div>
					{/each}
				</div>

				<div class="mt-4 flex flex-wrap items-center justify-end gap-2">
					<select
						class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
						bind:value={newSectionType}
					>
						<option value="hero">Hero</option>
						<option value="products">Products</option>
						<option value="promo_cards">Promo cards</option>
						<option value="badges">Badges</option>
					</select>
					<Button
						variant="primary"
						size="large"
						type="button"
						disabled={draft.homepage_sections.length >= maxHomepageSections}
						onclick={() => {
							if (draft.homepage_sections.length >= maxHomepageSections) {
								return;
							}
							draft.homepage_sections = [...draft.homepage_sections, createSection(newSectionType)];
							syncManualProductInputFromDraft();
						}}
					>
						<i class="bi bi-plus-lg mr-1"></i>
						Add section
					</Button>
				</div>
			</section>

			<section class="rounded-xl border border-gray-200 p-4 dark:border-gray-800">
				<h3
					class="text-sm font-semibold tracking-[0.18em] text-gray-500 uppercase dark:text-gray-400"
				>
					Footer
				</h3>
				<div class="mt-4 grid gap-3 md:grid-cols-3">
					<TextInput placeholder="Brand name" bind:value={draft.footer.brand_name} />
					<TextInput placeholder="Tagline" bind:value={draft.footer.tagline} />
					<TextInput placeholder="Copyright text" bind:value={draft.footer.copyright} />
				</div>
				<TextInput
					class="mt-3"
					placeholder="Bottom notice"
					bind:value={draft.footer.bottom_notice}
				/>

				<div class="mt-5">
					<div class="mb-3 flex items-center justify-between">
						<p
							class="text-xs font-semibold tracking-[0.18em] text-gray-400 uppercase dark:text-gray-500"
						>
							Footer columns
						</p>
						<IconButton
							variant="primary"
							type="button"
							disabled={draft.footer.columns.length >= maxFooterColumns}
							onclick={() => {
								if (draft.footer.columns.length >= maxFooterColumns) {
									return;
								}
								draft.footer.columns = [...draft.footer.columns, createEmptyFooterColumn()];
							}}
							aria-label="Add footer column"
						>
							<i class="bi bi-plus-lg"></i>
						</IconButton>
					</div>
					<div class="grid gap-4 lg:grid-cols-2">
						{#each draft.footer.columns as column, columnIndex (columnIndex)}
							<div class="rounded-lg border border-gray-200 p-3 dark:border-gray-800">
								<div class="flex items-center justify-between gap-3">
									<TextInput class="flex-1" placeholder="Column title" bind:value={column.title} />
									<IconButton
										variant="danger"
										type="button"
										disabled={draft.footer.columns.length <= 1}
										onclick={() => {
											if (draft.footer.columns.length <= 1) {
												return;
											}
											draft.footer.columns = draft.footer.columns.filter(
												(_, idx) => idx !== columnIndex
											);
										}}
										aria-label="Remove footer column"
									>
										<i class="bi bi-trash"></i>
									</IconButton>
								</div>

								<div class="mt-3 space-y-2">
									{#each column.links as link, linkIndex (linkIndex)}
										<div class="flex items-center gap-2">
											<TextInput class="flex-1" placeholder="Link label" bind:value={link.label} />
											<TextInput class="flex-1" placeholder="Link URL" bind:value={link.url} />
											<IconButton
												variant="danger"
												type="button"
												disabled={column.links.length <= 1}
												onclick={() => {
													if (column.links.length <= 1) {
														return;
													}
													column.links = column.links.filter((_, idx) => idx !== linkIndex);
													draft = cloneStorefrontSettings(draft);
												}}
												aria-label="Remove footer link"
											>
												<i class="bi bi-dash-lg"></i>
											</IconButton>
										</div>
									{/each}
								</div>
								<div class="mt-3 flex justify-end">
									<IconButton
										variant="primary"
										type="button"
										disabled={column.links.length >= maxFooterLinksPerColumn}
										onclick={() => {
											if (column.links.length >= maxFooterLinksPerColumn) {
												return;
											}
											column.links = [...column.links, createEmptyLink()];
											draft = cloneStorefrontSettings(draft);
										}}
										aria-label="Add footer link"
									>
										<i class="bi bi-plus-lg"></i>
									</IconButton>
								</div>
							</div>
						{/each}
					</div>
				</div>

				<div class="mt-5">
					<div class="mb-3 flex items-center justify-between">
						<p
							class="text-xs font-semibold tracking-[0.18em] text-gray-400 uppercase dark:text-gray-500"
						>
							Social links
						</p>
						<IconButton
							variant="primary"
							type="button"
							disabled={draft.footer.social_links.length >= maxSocialLinks}
							onclick={() => {
								if (draft.footer.social_links.length >= maxSocialLinks) {
									return;
								}
								draft.footer.social_links = [...draft.footer.social_links, createEmptyLink()];
							}}
							aria-label="Add social link"
						>
							<i class="bi bi-plus-lg"></i>
						</IconButton>
					</div>
					<div class="space-y-2">
						{#each draft.footer.social_links as socialLink, index (index)}
							<div class="flex items-center gap-2">
								<TextInput
									class="flex-1"
									placeholder="Social label"
									bind:value={socialLink.label}
								/>
								<TextInput class="flex-1" placeholder="Social URL" bind:value={socialLink.url} />
								<IconButton
									variant="danger"
									type="button"
									onclick={() => {
										draft.footer.social_links = draft.footer.social_links.filter(
											(_, linkIndex) => linkIndex !== index
										);
									}}
									aria-label="Remove social link"
								>
									<i class="bi bi-dash-lg"></i>
								</IconButton>
							</div>
						{/each}
					</div>
				</div>
			</section>
		</div>
	{/if}
</div>

{#if showInlineUnsavedNotice && hasUnsavedChanges}
	<div
		class="fixed bottom-4 left-1/2 z-50 -translate-x-1/2 rounded-full border border-gray-300 bg-gray-100 px-3 py-1.5 text-xs text-gray-700 shadow-sm dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200"
	>
		<div class="flex items-center gap-2">
			<span>You have unsaved storefront changes.</span>
			<Button
				variant="regular"
				size="small"
				style="pill"
				type="button"
				onclick={saveStorefrontSettings}
				disabled={saving || uploadingHero}
			>
				<i class="bi bi-floppy-fill"></i>
				{saving ? "Saving..." : "Save"}
			</Button>
		</div>
	</div>
{/if}
