<script lang="ts">
	import { type API } from "$lib/api";
	import { type SavedAddressModel, type SavedPaymentMethodModel } from "$lib/models";
	import Alert from "$lib/components/alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import { uploadMediaFiles } from "$lib/media";
	import { getProfile, userStore } from "$lib/user";
	import { getContext, onDestroy, onMount } from "svelte";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");

	let loading = $state(true);
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
	let email = $state("");
	let username = $state("");
	let profilePhotoUrl = $state<string | null>(null);
	let selectedFile = $state<File | null>(null);
	let previewUrl = $state<string | null>(null);
	let uploading = $state(false);
	let removing = $state(false);
	let authChecked = $state(false);
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

	async function loadProfile() {
		authChecked = true;
		isAuthenticated = await api.refreshAuthState();
		if (!isAuthenticated) {
			loading = false;
			return;
		}

		loading = true;
		pageError = "";
		try {
			const user = await getProfile(api);
			if (!user) {
				pageError = "Unable to load your profile. Please log in again.";
				return;
			}

			userStore.setUser(user);
			name = user.name ?? "";
			currency = user.currency ?? "USD";
			email = user.email;
			username = user.username;
			profilePhotoUrl = user.profile_photo_url;

			const [savedMethods, savedAddresses] = await Promise.all([
				api.listSavedPaymentMethods(),
				api.listSavedAddresses(),
			]);
			paymentMethods = savedMethods;
			addresses = savedAddresses;
		} catch (err) {
			console.error(err);
			pageError = "Unable to load your profile. Please try again.";
		} finally {
			loading = false;
		}
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		accountStatus = "";
		accountError = "";

		try {
			await api.updateProfile({
				name: name.trim() || undefined,
				currency: currency.trim() || undefined,
			});
			await loadProfile();
			accountStatus = "Profile updated.";
		} catch (err) {
			console.error(err);
			accountError = "Could not update profile. Please try again.";
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
			await loadProfile();
			photoStatus = "Profile photo updated.";
			clearPreview();
		} catch (err) {
			console.error(err);
			const error = err as { status?: number; body?: { error?: string } };
			if (error.status === 409 && error.body?.error === "Media is still processing") {
				photoError = "Photo is still processing. Please try again in a moment.";
			} else if (error.status === 422 && error.body?.error) {
				photoError = error.body.error;
			} else {
				photoError = error.body?.error ?? "Could not upload the photo. Please try again.";
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
			await loadProfile();
			photoStatus = "Profile photo removed.";
		} catch (err) {
			console.error(err);
			photoError = "Could not remove the photo.";
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
			paymentStatus = "Payment method saved.";
		} catch (err) {
			console.error(err);
			const error = err as { body?: { error?: string } };
			paymentError = error.body?.error ?? "Could not save payment method.";
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
			paymentStatus = "Payment method removed.";
		} catch (err) {
			console.error(err);
			paymentError = "Could not remove payment method.";
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
			paymentStatus = "Default payment method updated.";
		} catch (err) {
			console.error(err);
			paymentError = "Could not set default payment method.";
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
			addressStatus = "Address saved.";
		} catch (err) {
			console.error(err);
			const error = err as { body?: { error?: string } };
			addressError = error.body?.error ?? "Could not save address.";
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
			addressStatus = "Address removed.";
		} catch (err) {
			console.error(err);
			addressError = "Could not remove address.";
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
			addressStatus = "Default address updated.";
		} catch (err) {
			console.error(err);
			addressError = "Could not set default address.";
		} finally {
			busyAction = false;
		}
	}

	onMount(loadProfile);
	onDestroy(clearPreview);
</script>

<section>
	<div class="mx-auto max-w-5xl px-4 py-10">
		<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Profile</h1>

		{#if !authChecked}
			<div class="mt-6 grid gap-6 md:grid-cols-[280px_1fr]">
				<div
					class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				></div>
				<div
					class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				></div>
			</div>
		{:else if !isAuthenticated}
			<p class="mt-4 text-gray-600 dark:text-gray-300">
				Please
				<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
					log in
				</a>
				to view your profile.
			</p>
		{:else if loading}
			<div class="mt-6 grid gap-6 md:grid-cols-[280px_1fr]">
				<div
					class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				></div>
				<div
					class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				></div>
			</div>
		{:else}
			<div class="mt-8 grid items-start gap-6 md:grid-cols-[280px_1fr]">
				<div
					class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="flex flex-col items-center text-center">
						<div
							class="h-28 w-28 overflow-hidden rounded-full border border-gray-200 bg-gray-100 shadow-sm dark:border-gray-700 dark:bg-gray-800"
						>
							{#if previewUrl}
								<img src={previewUrl} alt="Profile preview" class="h-full w-full object-cover" />
							{:else if profilePhotoUrl}
								<img src={profilePhotoUrl} alt="Profile" class="h-full w-full object-cover" />
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
						<label
							class="inline-flex w-full cursor-pointer items-center justify-between rounded-lg border border-gray-300 bg-gray-200 px-4 py-2 transition-[background-color,border-color] duration-200 hover:border-gray-200 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-700 hover:dark:border-gray-700 hover:dark:bg-gray-800"
						>
							<input type="file" accept="image/*" class="hidden" onchange={handleFileChange} />
							<i class="bi bi-folder-fill"></i>
							Choose photo
							<span></span>
						</label>
						<Button
							type="button"
							variant="primary"
							class="w-full"
							disabled={!selectedFile || uploading}
							onclick={uploadPhoto}
						>
							<i class="bi bi-upload float-left"></i>
							{uploading ? "Uploading..." : "Upload photo"}
						</Button>
						<Button
							type="button"
							variant="warning"
							class="w-full"
							disabled={!profilePhotoUrl || removing}
							onclick={removePhoto}
						>
							<i class="bi bi-trash-fill float-left"></i>
							{removing ? "Removing..." : "Remove photo"}
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
				</div>

				<div class="space-y-6">
					{#if pageError}
						<Alert
							message={pageError}
							tone="error"
							icon="bi-x-circle-fill"
							onClose={() => (pageError = "")}
						/>
					{/if}
					<div
						class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
					>
						<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Account details</h3>
						<form class="mt-6 space-y-4" onsubmit={submit}>
							<div class="grid gap-4 md:grid-cols-2">
								<div>
									<label for="username" class="text-sm font-medium text-gray-600 dark:text-gray-300">
										Username
									</label>
									<TextInput id="username" class="mt-1" type="text" value={username} readonly />
								</div>
								<div>
									<label for="email" class="text-sm font-medium text-gray-600 dark:text-gray-300">
										Email
									</label>
									<TextInput id="email" class="mt-1" type="email" value={email} readonly />
								</div>
							</div>

							<div class="grid gap-4 md:grid-cols-2">
								<div>
									<label for="name" class="text-sm font-medium text-gray-600 dark:text-gray-300">
										Name
									</label>
									<TextInput
										id="name"
										class="mt-1"
										type="text"
										bind:value={name}
										placeholder="Your name"
									/>
								</div>
								<div>
									<label for="currency" class="text-sm font-medium text-gray-600 dark:text-gray-300">
										Preferred currency
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
									Save changes
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
					</div>

					<div class="grid items-start gap-6 xl:grid-cols-2">
						<div
							class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
						>
							<div class="flex items-center justify-between">
								<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Saved payment methods</h3>
							</div>
							<form class="mt-4 grid gap-3" onsubmit={addPaymentMethod}>
								<TextInput bind:value={cardholderName} placeholder="Cardholder name" />
								<TextInput bind:value={cardNumber} placeholder="Card number" />
								<div class="grid grid-cols-2 gap-3">
									<NumberInput bind:value={expMonth} placeholder="Exp month" min={1} max={12} />
									<NumberInput bind:value={expYear} placeholder="Exp year" min={2024} max={2200} />
								</div>
								<TextInput bind:value={paymentNickname} placeholder="Nickname (optional)" />
								<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
									<input type="checkbox" bind:checked={setPaymentDefault} />
									Set as default
								</label>
								<Button type="submit" variant="primary" disabled={busyAction}>
									<i class="bi bi-plus-lg mr-1"></i>
									Save payment method
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
									<p class="text-sm text-gray-500 dark:text-gray-400">No saved payment methods.</p>
								{:else}
									{#each paymentMethods as method (method.id)}
										<div class="rounded-xl border border-gray-200 p-3 dark:border-gray-800">
											<div class="flex items-center justify-between gap-3">
												<div>
													<p class="font-medium text-gray-900 dark:text-gray-100">
														{method.nickname || `${method.brand} •••• ${method.last4}`}
														{method.is_default ? " (Default)" : ""}
													</p>
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
															title="Set as default"
															aria-label="Set as default payment method"
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
														title="Delete payment method"
														aria-label="Delete payment method"
													>
														<i class="bi bi-trash-fill"></i>
													</IconButton>
												</div>
											</div>
										</div>
									{/each}
								{/if}
							</div>
						</div>

						<div
							class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
						>
							<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Saved addresses</h3>
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
									Set as default
								</label>
								<Button type="submit" variant="primary" disabled={busyAction}>
									<i class="bi bi-plus-lg mr-1"></i>
									Save address
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
									<p class="text-sm text-gray-500 dark:text-gray-400">No saved addresses.</p>
								{:else}
									{#each addresses as address (address.id)}
										<div class="rounded-xl border border-gray-200 p-3 dark:border-gray-800">
											<div class="flex items-center justify-between gap-3">
												<div>
													<p class="font-medium text-gray-900 dark:text-gray-100">
														{address.label || address.line1}{address.is_default ? " (Default)" : ""}
													</p>
													<p class="text-xs text-gray-500 dark:text-gray-400">
														{address.full_name}, {address.line1}{address.line2 ? `, ${address.line2}` : ""}, {address.city}, {address.state}, {address.postal_code}, {address.country}
													</p>
												</div>
												<div class="flex gap-2">
													{#if !address.is_default}
														<IconButton
															size="sm"
															disabled={busyAction}
															onclick={() => setDefaultAddress(address.id)}
															title="Set as default"
															aria-label="Set as default address"
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
														title="Delete address"
														aria-label="Delete address"
													>
														<i class="bi bi-trash-fill"></i>
													</IconButton>
												</div>
											</div>
										</div>
									{/each}
								{/if}
							</div>
						</div>
					</div>
				</div>
			</div>

		{/if}
	</div>
</section>
