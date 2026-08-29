<script lang="ts">
	import { type API } from "$lib/api";
	import {
		type SavedAddressModel,
		type SavedPaymentMethodModel,
		type UserModel,
	} from "$lib/models";
	import Alert from "$lib/components/Alert.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import ButtonInput from "$lib/components/ButtonInput.svelte";
	import Card from "$lib/components/Card.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import { uploadMediaFiles } from "$lib/media";
	import { getLocalizedApiErrorMessage } from "$lib/api/errors";
	import { getContext, onDestroy } from "svelte";
	import { resolve } from "$app/paths";
	import type { PageData } from "./$types";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";

	const api: API = getContext("api");
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);
	interface Props {
		data: PageData;
	}
	let { data }: Props = $props();

	let pageError = $state("");
	let accountError = $state("");
	let accountStatus = $state("");
	let photoError = $state("");
	let photoStatus = $state("");
	let paymentError = $state("");
	let paymentStatus = $state("");
	let addressError = $state("");
	let addressStatus = $state("");
	let name = $state("");
	let currency = $state("USD");
	let locale = $state("en-US");
	let email = $state("");
	let username = $state("");
	let profilePhotoUrl = $state<string | null>(null);
	let selectedFile = $state<File | null>(null);
	let previewUrl = $state<string | null>(null);
	let uploading = $state(false);
	let removing = $state(false);
	let isAuthenticated = $state(false);
	let busyAction = $state(false);

	let paymentMethods = $state<SavedPaymentMethodModel[]>([]);
	let addresses = $state<SavedAddressModel[]>([]);

	let cardholderName = $state("");
	let cardNumber = $state("");
	let expMonth = $state("");
	let expYear = $state("");
	let paymentNickname = $state("");
	let setPaymentDefault = $state(false);

	let addressLabel = $state("");
	let fullName = $state("");
	let line1 = $state("");
	let line2 = $state("");
	let city = $state("");
	let region = $state("");
	let postalCode = $state("");
	let country = $state("US");
	let phone = $state("");
	let setAddressDefault = $state(false);

	function clearPreview() {
		if (previewUrl) {
			URL.revokeObjectURL(previewUrl);
		}
		previewUrl = null;
		selectedFile = null;
	}

	function handleFileChange(event: Event) {
		const target = event.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) {
			clearPreview();
			return;
		}
		clearPreview();
		selectedFile = file;
		previewUrl = URL.createObjectURL(file);
	}

	function applyProfileData(profile: UserModel | null) {
		if (!profile) {
			name = "";
			currency = "USD";
			locale = localization.locale;
			email = "";
			username = "";
			profilePhotoUrl = null;
			return;
		}

		name = profile.name ?? "";
		currency = profile.currency ?? "USD";
		locale = profile.locale ?? localization.locale;
		email = profile.email;
		username = profile.username;
		profilePhotoUrl = profile.profile_photo_url;
	}

	async function refreshProfileData() {
		try {
			const [profile, savedMethods, savedAddresses] = await Promise.all([
				api.getProfile(),
				api.listSavedPaymentMethods(),
				api.listSavedAddresses(),
			]);

			isAuthenticated = true;
			applyProfileData(profile);
			paymentMethods = savedMethods;
			addresses = savedAddresses;
			pageError = "";
		} catch (err) {
			const error = err as { status?: number };
			if (error.status === 401) {
				isAuthenticated = false;
				applyProfileData(null);
				paymentMethods = [];
				addresses = [];
				pageError = "";
				return;
			}
			console.error(err);
			pageError = $localization.translate(
				"storefront.account.profile_load_error",
				"Unable to load your profile. Please try again."
			);
		}
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		accountStatus = "";
		accountError = "";

		try {
			const profile = await api.updateProfile({
				name: name.trim() || undefined,
				currency: currency.trim() || undefined,
				locale,
			});
			if (profile.locale !== localization.locale) {
				await localization.setLocale(profile.locale);
			}
			await refreshProfileData();
			accountStatus = $localization.translate(
				"storefront.account.profile_updated",
				"Profile updated."
			);
		} catch (err) {
			console.error(err);
			accountError = getLocalizedApiErrorMessage(
				err,
				(key, source, parameters) => $localization.translate(key, source, parameters),
				$localization.translate(
					"storefront.account.profile_update_error",
					"Could not update profile. Please try again."
				)
			);
		}
	}

	async function uploadPhoto() {
		if (!selectedFile) {
			return;
		}

		uploading = true;
		photoError = "";
		photoStatus = "";

		try {
			const [mediaId] = await uploadMediaFiles(api, [selectedFile]);
			if (!mediaId) {
				throw new Error("Upload failed");
			}
			await api.attachProfilePhoto(mediaId);
			await refreshProfileData();
			photoStatus = $localization.translate(
				"storefront.account.photo_updated",
				"Profile photo updated."
			);
			clearPreview();
		} catch (err) {
			console.error(err);
			const error = err as { status?: number };
			if (error.status === 409) {
				photoError = $localization.translate(
					"storefront.account.photo_processing",
					"Photo is still processing. Please try again in a moment."
				);
			} else {
				photoError = getLocalizedApiErrorMessage(
					err,
					(key, source, parameters) => $localization.translate(key, source, parameters),
					$localization.translate(
						"storefront.account.photo_upload_error",
						"Could not upload the photo. Please try again."
					)
				);
			}
		} finally {
			uploading = false;
		}
	}

	async function removePhoto() {
		if (!profilePhotoUrl) {
			return;
		}

		removing = true;
		photoError = "";
		photoStatus = "";

		try {
			await api.removeProfilePhoto();
			await refreshProfileData();
			photoStatus = $localization.translate(
				"storefront.account.photo_removed",
				"Profile photo removed."
			);
		} catch (err) {
			console.error(err);
			photoError = $localization.translate(
				"storefront.account.photo_remove_error",
				"Could not remove the photo."
			);
		} finally {
			removing = false;
		}
	}

	async function addPaymentMethod(event: SubmitEvent) {
		event.preventDefault();
		busyAction = true;
		paymentError = "";
		paymentStatus = "";
		try {
			await api.createSavedPaymentMethod({
				cardholder_name: cardholderName.trim(),
				card_number: cardNumber,
				exp_month: Number(expMonth),
				exp_year: Number(expYear),
				nickname: paymentNickname.trim() || undefined,
				set_default: setPaymentDefault,
			});
			cardholderName = "";
			cardNumber = "";
			expMonth = "";
			expYear = "";
			paymentNickname = "";
			setPaymentDefault = false;
			paymentMethods = await api.listSavedPaymentMethods();
			paymentStatus = $localization.translate(
				"storefront.account.payment_saved",
				"Payment method saved."
			);
		} catch (err) {
			console.error(err);
			paymentError = getLocalizedApiErrorMessage(
				err,
				(key, source, parameters) => $localization.translate(key, source, parameters),
				$localization.translate(
					"storefront.account.payment_save_error",
					"Could not save payment method."
				)
			);
		} finally {
			busyAction = false;
		}
	}

	async function deletePaymentMethod(id: number) {
		busyAction = true;
		paymentError = "";
		paymentStatus = "";
		try {
			await api.deleteSavedPaymentMethod(id);
			paymentMethods = await api.listSavedPaymentMethods();
			paymentStatus = $localization.translate(
				"storefront.account.payment_removed",
				"Payment method removed."
			);
		} catch (err) {
			console.error(err);
			paymentError = $localization.translate(
				"storefront.account.payment_remove_error",
				"Could not remove payment method."
			);
		} finally {
			busyAction = false;
		}
	}

	async function setDefaultPaymentMethod(id: number) {
		busyAction = true;
		paymentError = "";
		paymentStatus = "";
		try {
			await api.setDefaultPaymentMethod(id);
			paymentMethods = await api.listSavedPaymentMethods();
			paymentStatus = $localization.translate(
				"storefront.account.payment_default_updated",
				"Default payment method updated."
			);
		} catch (err) {
			console.error(err);
			paymentError = $localization.translate(
				"storefront.account.payment_default_error",
				"Could not set default payment method."
			);
		} finally {
			busyAction = false;
		}
	}

	async function addAddress(event: SubmitEvent) {
		event.preventDefault();
		busyAction = true;
		addressError = "";
		addressStatus = "";
		try {
			await api.createSavedAddress({
				label: addressLabel.trim() || undefined,
				full_name: fullName.trim(),
				line1: line1.trim(),
				line2: line2.trim() || undefined,
				city: city.trim(),
				state: region.trim() || undefined,
				postal_code: postalCode.trim(),
				country: country.trim().toUpperCase(),
				phone: phone.trim() || undefined,
				set_default: setAddressDefault,
			});
			addressLabel = "";
			fullName = "";
			line1 = "";
			line2 = "";
			city = "";
			region = "";
			postalCode = "";
			country = "US";
			phone = "";
			setAddressDefault = false;
			addresses = await api.listSavedAddresses();
			addressStatus = $localization.translate("storefront.account.address_saved", "Address saved.");
		} catch (err) {
			console.error(err);
			addressError = getLocalizedApiErrorMessage(
				err,
				(key, source, parameters) => $localization.translate(key, source, parameters),
				$localization.translate("storefront.account.address_save_error", "Could not save address.")
			);
		} finally {
			busyAction = false;
		}
	}

	async function deleteAddress(id: number) {
		busyAction = true;
		addressError = "";
		addressStatus = "";
		try {
			await api.deleteSavedAddress(id);
			addresses = await api.listSavedAddresses();
			addressStatus = $localization.translate(
				"storefront.account.address_removed",
				"Address removed."
			);
		} catch (err) {
			console.error(err);
			addressError = $localization.translate(
				"storefront.account.address_remove_error",
				"Could not remove address."
			);
		} finally {
			busyAction = false;
		}
	}

	async function setDefaultAddress(id: number) {
		busyAction = true;
		addressError = "";
		addressStatus = "";
		try {
			await api.setDefaultAddress(id);
			addresses = await api.listSavedAddresses();
			addressStatus = $localization.translate(
				"storefront.account.address_default_updated",
				"Default address updated."
			);
		} catch (err) {
			console.error(err);
			addressError = $localization.translate(
				"storefront.account.address_default_error",
				"Could not set default address."
			);
		} finally {
			busyAction = false;
		}
	}

	$effect(() => {
		isAuthenticated = data.isAuthenticated;
		pageError = data.errorMessage;
		applyProfileData(data.profile);
		paymentMethods = data.savedPaymentMethods;
		addresses = data.savedAddresses;
	});

	onDestroy(clearPreview);
</script>

<section>
	<div class="mx-auto max-w-5xl px-4 py-10">
		<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">
			{$localization.translate("storefront.account.profile", "Profile")}
		</h1>

		{#if !isAuthenticated}
			<p class="mt-4 text-gray-600 dark:text-gray-300">
				{$localization.translate("storefront.account.please", "Please")}
				<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
					{$localization.translate("storefront.account.log_in_lower", "log in")}
				</a>
				{$localization.translate("storefront.account.view_profile_suffix", "to view your profile.")}
			</p>
		{:else}
			<div class="mt-8 grid items-start gap-6 md:grid-cols-[280px_1fr]">
				<Card padding="lg">
					<div class="flex flex-col items-center text-center">
						<div
							class="h-28 w-28 overflow-hidden rounded-full border border-gray-200 bg-gray-100 shadow-sm dark:border-gray-700 dark:bg-gray-800"
						>
							{#if previewUrl}
								<img
									src={previewUrl}
									alt={$localization.translate(
										"storefront.account.profile_preview",
										"Profile preview"
									)}
									class="h-full w-full object-cover"
								/>
							{:else if profilePhotoUrl}
								<img
									src={profilePhotoUrl}
									alt={$localization.translate("storefront.account.profile_photo", "Profile photo")}
									class="h-full w-full object-cover"
								/>
							{:else}
								<div
									class="flex h-full w-full items-center justify-center text-2xl font-semibold text-gray-500 dark:text-gray-300"
								>
									{(name || username || "?").slice(0, 1).toUpperCase()}
								</div>
							{/if}
						</div>
						<h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
							{name || username}
						</h2>
						<p class="text-sm text-gray-500 dark:text-gray-400">@{username}</p>
						<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{email}</p>
					</div>

					<div class="mt-6 space-y-3 text-sm text-gray-600 dark:text-gray-300">
						<ButtonInput
							class="w-full"
							type="file"
							accept="image/*"
							onchange={handleFileChange}
							variant="regular"
						>
							<i class="bi bi-folder-fill mr-2"></i>
							{$localization.translate("storefront.account.choose_photo", "Choose photo")}
						</ButtonInput>
						<Button
							type="button"
							variant="primary"
							class="w-full"
							disabled={!selectedFile || uploading}
							onclick={uploadPhoto}
						>
							<i class="bi bi-upload mr-2"></i>
							{uploading
								? $localization.translate("storefront.account.uploading", "Uploading...")
								: $localization.translate("storefront.account.upload_photo", "Upload photo")}
						</Button>
						<Button
							type="button"
							variant="warning"
							class="w-full"
							disabled={!profilePhotoUrl || removing}
							onclick={removePhoto}
						>
							<i class="bi bi-trash-fill mr-2"></i>
							{removing
								? $localization.translate("storefront.account.removing", "Removing...")
								: $localization.translate("storefront.account.remove_photo", "Remove photo")}
						</Button>
						{#if photoError}
							<Alert
								message={photoError}
								tone="error"
								icon="bi-x-circle-fill"
								onClose={() => (photoError = "")}
							/>
						{/if}
						{#if photoStatus}
							<Alert
								message={photoStatus}
								tone="success"
								icon="bi-check-circle-fill"
								onClose={() => (photoStatus = "")}
							/>
						{/if}
					</div>
				</Card>

				<div class="space-y-6">
					{#if pageError}
						<Alert
							message={pageError}
							tone="error"
							icon="bi-x-circle-fill"
							onClose={() => (pageError = "")}
						/>
					{/if}
					<Card padding="lg">
						<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
							{$localization.translate("storefront.account.details", "Account details")}
						</h3>
						<form class="mt-6 space-y-4" onsubmit={submit}>
							<div class="grid gap-4 md:grid-cols-2">
								<div>
									<label
										for="username"
										class="text-sm font-medium text-gray-600 dark:text-gray-300"
									>
										{$localization.translate("storefront.account.username", "Username")}
									</label>
									<TextInput id="username" class="mt-1" type="text" value={username} readonly />
								</div>
								<div>
									<label for="email" class="text-sm font-medium text-gray-600 dark:text-gray-300">
										{$localization.translate("storefront.account.email", "Email")}
									</label>
									<TextInput id="email" class="mt-1" type="email" value={email} readonly />
								</div>
							</div>

							<div>
								<label for="locale" class="text-sm font-medium text-gray-600 dark:text-gray-300">
									{$localization.translate("storefront.account.locale", "Language")}
								</label>
								<Dropdown id="locale" class="mt-1" bind:value={locale}>
									{#each localization.locales.filter((candidate) => candidate.is_enabled) as candidate (candidate.code)}
										<option value={candidate.code}>{candidate.name}</option>
									{/each}
								</Dropdown>
							</div>

							<div class="grid gap-4 md:grid-cols-2">
								<div>
									<label for="name" class="text-sm font-medium text-gray-600 dark:text-gray-300">
										{$localization.translate("storefront.account.name", "Name")}
									</label>
									<TextInput
										id="name"
										class="mt-1"
										type="text"
										bind:value={name}
										placeholder={$localization.translate(
											"storefront.account.your_name",
											"Your name"
										)}
									/>
								</div>
								<div>
									<label
										for="currency"
										class="text-sm font-medium text-gray-600 dark:text-gray-300"
									>
										{$localization.translate(
											"storefront.account.preferred_currency",
											"Preferred currency"
										)}
									</label>
									<TextInput
										id="currency"
										class="mt-1"
										type="text"
										bind:value={currency}
										placeholder="USD"
									/>
								</div>
							</div>

							<div class="flex justify-end">
								<Button variant="primary" size="large" type="submit">
									<i class="bi bi-floppy-fill mr-1"></i>
									{$localization.translate("storefront.account.save", "Save changes")}
								</Button>
							</div>
							{#if accountError}
								<Alert
									message={accountError}
									tone="error"
									icon="bi-x-circle-fill"
									onClose={() => (accountError = "")}
								/>
							{/if}
							{#if accountStatus}
								<Alert
									message={accountStatus}
									tone="success"
									icon="bi-check-circle-fill"
									onClose={() => (accountStatus = "")}
								/>
							{/if}
						</form>
					</Card>

					<div class="grid items-start gap-6 xl:grid-cols-2">
						<Card padding="lg">
							<div class="flex items-center justify-between">
								<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
									{$localization.translate(
										"storefront.account.saved_payment_methods",
										"Saved payment methods"
									)}
								</h3>
							</div>
							<form class="mt-4 grid gap-3" onsubmit={addPaymentMethod}>
								<TextInput
									bind:value={cardholderName}
									placeholder={$localization.translate(
										"storefront.account.cardholder_name",
										"Cardholder name"
									)}
								/>
								<TextInput
									bind:value={cardNumber}
									placeholder={$localization.translate(
										"storefront.account.card_number",
										"Card number"
									)}
								/>
								<div class="grid grid-cols-2 gap-3">
									<NumberInput
										bind:value={expMonth}
										placeholder={$localization.translate(
											"storefront.account.expiration_month",
											"Exp month"
										)}
										min={1}
										max={12}
									/>
									<NumberInput
										bind:value={expYear}
										placeholder={$localization.translate(
											"storefront.account.expiration_year",
											"Exp year"
										)}
										min={2024}
										max={2200}
									/>
								</div>
								<TextInput bind:value={paymentNickname} placeholder="Nickname (optional)" />
								<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
									<input type="checkbox" bind:checked={setPaymentDefault} />
									{$localization.translate("storefront.account.set_as_default", "Set as default")}
								</label>
								<Button type="submit" variant="primary" disabled={busyAction}>
									<i class="bi bi-plus-lg mr-1"></i>
									{$localization.translate(
										"storefront.account.save_payment_method",
										"Save payment method"
									)}
								</Button>
								{#if paymentError}
									<Alert
										message={paymentError}
										tone="error"
										icon="bi-x-circle-fill"
										onClose={() => (paymentError = "")}
									/>
								{/if}
								{#if paymentStatus}
									<Alert
										message={paymentStatus}
										tone="success"
										icon="bi-check-circle-fill"
										onClose={() => (paymentStatus = "")}
									/>
								{/if}
							</form>

							<div class="mt-4 space-y-2">
								{#if paymentMethods.length === 0}
									<p class="text-sm text-gray-500 dark:text-gray-400">
										{$localization.translate(
											"storefront.account.no_payment_methods",
											"No saved payment methods."
										)}
									</p>
								{:else}
									{#each paymentMethods as method (method.id)}
										<Card radius="xl" padding="sm" shadow="none" class="dark:bg-transparent">
											<div class="flex items-center justify-between gap-3">
												<div>
													<div class="flex items-center gap-2">
														<p class="font-medium text-gray-900 dark:text-gray-100">
															{method.nickname || `${method.brand} •••• ${method.last4}`}
														</p>
														{#if method.is_default}
															<Badge tone="success" size="xs" class="tracking-wide uppercase">
																{$localization.translate("storefront.account.default", "Default")}
															</Badge>
														{/if}
													</div>
													<p class="text-xs text-gray-500 dark:text-gray-400">
														{method.brand} •••• {method.last4} · Expires {method.exp_month}/{method.exp_year}
													</p>
												</div>
												<div class="flex gap-2">
													{#if !method.is_default}
														<IconButton
															size="sm"
															disabled={busyAction}
															onclick={() => setDefaultPaymentMethod(method.id)}
															title={$localization.translate(
																"storefront.account.set_as_default",
																"Set as default"
															)}
															aria-label={$localization.translate(
																"storefront.account.set_default_payment_method",
																"Set as default payment method"
															)}
															variant="primary"
														>
															<i class="bi bi-check-circle-fill"></i>
														</IconButton>
													{/if}
													<IconButton
														variant="danger"
														size="sm"
														disabled={busyAction}
														onclick={() => deletePaymentMethod(method.id)}
														title={$localization.translate(
															"storefront.account.delete_payment_method",
															"Delete payment method"
														)}
														aria-label={$localization.translate(
															"storefront.account.delete_payment_method",
															"Delete payment method"
														)}
													>
														<i class="bi bi-trash-fill"></i>
													</IconButton>
												</div>
											</div>
										</Card>
									{/each}
								{/if}
							</div>
						</Card>

						<Card padding="lg">
							<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
								{$localization.translate("storefront.account.saved_addresses", "Saved addresses")}
							</h3>
							<form class="mt-4 grid gap-3" onsubmit={addAddress}>
								<TextInput bind:value={addressLabel} placeholder="Label (optional, e.g. Home)" />
								<TextInput bind:value={fullName} placeholder="Full name" />
								<TextInput bind:value={line1} placeholder="Address line 1" />
								<TextInput bind:value={line2} placeholder="Address line 2 (optional)" />
								<div class="grid grid-cols-2 gap-3">
									<TextInput bind:value={city} placeholder="City" />
									<TextInput bind:value={region} placeholder="State / Province" />
								</div>
								<div class="grid grid-cols-2 gap-3">
									<TextInput bind:value={postalCode} placeholder="Postal code" />
									<TextInput bind:value={country} maxlength={2} placeholder="Country (US)" />
								</div>
								<TextInput bind:value={phone} placeholder="Phone (optional)" />
								<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
									<input type="checkbox" bind:checked={setAddressDefault} />
									{$localization.translate("storefront.account.set_as_default", "Set as default")}
								</label>
								<Button type="submit" variant="primary" disabled={busyAction}>
									<i class="bi bi-plus-lg mr-1"></i>
									{$localization.translate("storefront.account.save_address", "Save address")}
								</Button>
								{#if addressError}
									<Alert
										message={addressError}
										tone="error"
										icon="bi-x-circle-fill"
										onClose={() => (addressError = "")}
									/>
								{/if}
								{#if addressStatus}
									<Alert
										message={addressStatus}
										tone="success"
										icon="bi-check-circle-fill"
										onClose={() => (addressStatus = "")}
									/>
								{/if}
							</form>

							<div class="mt-4 space-y-2">
								{#if addresses.length === 0}
									<p class="text-sm text-gray-500 dark:text-gray-400">
										{$localization.translate(
											"storefront.account.no_addresses",
											"No saved addresses."
										)}
									</p>
								{:else}
									{#each addresses as address (address.id)}
										<Card radius="xl" padding="sm" shadow="none" class="dark:bg-transparent">
											<div class="flex items-center justify-between gap-3">
												<div>
													<div class="flex items-center gap-2">
														<p class="font-medium text-gray-900 dark:text-gray-100">
															{address.label || address.line1}
														</p>
														{#if address.is_default}
															<Badge tone="success" size="xs" class="tracking-wide uppercase">
																{$localization.translate("storefront.account.default", "Default")}
															</Badge>
														{/if}
													</div>
													<p class="text-xs text-gray-500 dark:text-gray-400">
														{address.full_name}, {address.line1}{address.line2
															? `, ${address.line2}`
															: ""}, {address.city}, {address.state}, {address.postal_code}, {address.country}
													</p>
												</div>
												<div class="flex gap-2">
													{#if !address.is_default}
														<IconButton
															size="sm"
															disabled={busyAction}
															onclick={() => setDefaultAddress(address.id)}
															title={$localization.translate(
																"storefront.account.set_as_default",
																"Set as default"
															)}
															aria-label={$localization.translate(
																"storefront.account.set_default_address",
																"Set as default address"
															)}
															variant="primary"
														>
															<i class="bi bi-check-circle-fill"></i>
														</IconButton>
													{/if}
													<IconButton
														variant="danger"
														size="sm"
														disabled={busyAction}
														onclick={() => deleteAddress(address.id)}
														title={$localization.translate(
															"storefront.account.delete_address",
															"Delete address"
														)}
														aria-label={$localization.translate(
															"storefront.account.delete_address",
															"Delete address"
														)}
													>
														<i class="bi bi-trash-fill"></i>
													</IconButton>
												</div>
											</div>
										</Card>
									{/each}
								{/if}
							</div>
						</Card>
					</div>
				</div>
			</div>
		{/if}
	</div>
</section>
