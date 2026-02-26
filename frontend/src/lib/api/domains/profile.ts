import type { components } from "$lib/api/generated/openapi";
import {
	parseProfile,
	parseSavedAddress,
	parseSavedPaymentMethod,
	type SavedAddressModel,
	type SavedPaymentMethodModel,
	type UserModel,
	type ProfileModel,
} from "$lib/models";

type RequestFn = <T>(
	method: string,
	path: string,
	data?: object,
	params?: Record<string, unknown>
) => Promise<T>;

type UpdateProfileRequest = components["schemas"]["UpdateProfileRequest"];
type CreateSavedPaymentMethodRequest = components["schemas"]["CreateSavedPaymentMethodRequest"];
type CreateSavedAddressRequest = components["schemas"]["CreateSavedAddressRequest"];
type MessageResponse = components["schemas"]["MessageResponse"];

export async function getProfile(request: RequestFn): Promise<UserModel> {
	const response = await request<ProfileModel>("GET", "/me/");
	return parseProfile(response);
}

export async function updateProfile(
	request: RequestFn,
	data: UpdateProfileRequest
): Promise<UserModel> {
	const response = await request<ProfileModel>("PATCH", "/me/", data);
	return parseProfile(response);
}

export async function listSavedPaymentMethods(
	request: RequestFn
): Promise<SavedPaymentMethodModel[]> {
	const response = await request<components["schemas"]["SavedPaymentMethod"][]>(
		"GET",
		"/me/payment-methods"
	);
	return response.map(parseSavedPaymentMethod);
}

export async function createSavedPaymentMethod(
	request: RequestFn,
	data: CreateSavedPaymentMethodRequest
): Promise<SavedPaymentMethodModel> {
	const response = await request<components["schemas"]["SavedPaymentMethod"]>(
		"POST",
		"/me/payment-methods",
		data
	);
	return parseSavedPaymentMethod(response);
}

export async function deleteSavedPaymentMethod(
	request: RequestFn,
	id: number
): Promise<MessageResponse> {
	return request("DELETE", `/me/payment-methods/${id}`);
}

export async function setDefaultPaymentMethod(
	request: RequestFn,
	id: number
): Promise<SavedPaymentMethodModel> {
	const response = await request<components["schemas"]["SavedPaymentMethod"]>(
		"PATCH",
		`/me/payment-methods/${id}/default`
	);
	return parseSavedPaymentMethod(response);
}

export async function listSavedAddresses(request: RequestFn): Promise<SavedAddressModel[]> {
	const response = await request<components["schemas"]["SavedAddress"][]>("GET", "/me/addresses");
	return response.map(parseSavedAddress);
}

export async function createSavedAddress(
	request: RequestFn,
	data: CreateSavedAddressRequest
): Promise<SavedAddressModel> {
	const response = await request<components["schemas"]["SavedAddress"]>(
		"POST",
		"/me/addresses",
		data
	);
	return parseSavedAddress(response);
}

export async function deleteSavedAddress(request: RequestFn, id: number): Promise<MessageResponse> {
	return request("DELETE", `/me/addresses/${id}`);
}

export async function setDefaultAddress(
	request: RequestFn,
	id: number
): Promise<SavedAddressModel> {
	const response = await request<components["schemas"]["SavedAddress"]>(
		"PATCH",
		`/me/addresses/${id}/default`
	);
	return parseSavedAddress(response);
}

export async function attachProfilePhoto(request: RequestFn, mediaId: string): Promise<UserModel> {
	const response = await request<ProfileModel>("POST", "/me/profile-photo", { media_id: mediaId });
	return parseProfile(response);
}

export async function removeProfilePhoto(request: RequestFn): Promise<UserModel> {
	const response = await request<ProfileModel>("DELETE", "/me/profile-photo");
	return parseProfile(response);
}
