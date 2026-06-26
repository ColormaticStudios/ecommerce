<!-- Generator: Widdershins v4.0.1 -->

<h1 id="ecommerce-api">Ecommerce API v0.2.0</h1>

> Scroll down for code samples, example requests and responses. Select a language for code samples from the tabs above or the mobile navigation menu.

API contract for backend and frontend type generation.

Base URLs:

* <a href="http://localhost:3000">http://localhost:3000</a>

# Authentication

* API Key (cookieAuth)
    - Parameter Name: **session_token**, in: cookie. 

- HTTP Authentication, scheme: bearer 

<h1 id="ecommerce-api-auth">auth</h1>

## register

<a id="opIdregister"></a>

> Code samples

```javascript
const inputBody = '{
  "username": "string",
  "email": "string",
  "password": "string",
  "name": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/auth/register',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/auth/register`

> Body parameter

```json
{
  "username": "string",
  "email": "string",
  "password": "string",
  "name": "string"
}
```

<h3 id="register-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|RegisterRequest|true|none|

<h3 id="register-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Registered|AuthResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict|Error|

<aside class="success">
This operation does not require authentication
</aside>

## login

<a id="opIdlogin"></a>

> Code samples

```javascript
const inputBody = '{
  "email": "string",
  "password": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/auth/login',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/auth/login`

> Body parameter

```json
{
  "email": "string",
  "password": "string"
}
```

<h3 id="login-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|LoginRequest|true|none|

<h3 id="login-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Authenticated|AuthResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|Error|

<aside class="success">
This operation does not require authentication
</aside>

## logout

<a id="opIdlogout"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/auth/logout',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/auth/logout`

<h3 id="logout-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Logged out|MessageResponse|

<aside class="success">
This operation does not require authentication
</aside>

## getAuthConfig

<a id="opIdgetAuthConfig"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/auth/config',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/auth/config`

<h3 id="getauthconfig-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Authentication configuration|AuthConfigResponse|

<aside class="success">
This operation does not require authentication
</aside>

## oidcLogin

<a id="opIdoidcLogin"></a>

> Code samples

```javascript

fetch('http://localhost:3000/api/v1/auth/oidc/login',
{
  method: 'GET'

})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/auth/oidc/login`

<h3 id="oidclogin-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|redirect|query|string|false|none|
|response_format|query|string|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|response_format|json|

<h3 id="oidclogin-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|302|[Found](https://tools.ietf.org/html/rfc7231#section-6.4.3)|Redirect to provider|None|

<aside class="success">
This operation does not require authentication
</aside>

## oidcCallback

<a id="opIdoidcCallback"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/auth/oidc/callback',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/auth/oidc/callback`

<h3 id="oidccallback-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|code|query|string|false|none|
|state|query|string|false|none|
|format|query|string|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|format|json|

<h3 id="oidccallback-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Callback as JSON|AuthResponse|
|302|[Found](https://tools.ietf.org/html/rfc7231#section-6.4.3)|Callback as redirect|None|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|Error|

<aside class="success">
This operation does not require authentication
</aside>

<h1 id="ecommerce-api-products">products</h1>

## List products

<a id="opIdlistProducts"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/products',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/products`

<h3 id="list-products-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|q|query|string|false|none|
|min_price|query|number(double)|false|none|
|max_price|query|number(double)|false|none|
|brand_slug|query|string|false|none|
|category_slug|query|array[string]|false|none|
|has_variant_stock|query|boolean|false|none|
|attribute|query|object|false|none|
|sort|query|string|false|none|
|order|query|string|false|none|
|page|query|integer|false|none|
|limit|query|integer|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|sort|price|
|sort|name|
|sort|created_at|
|order|asc|
|order|desc|

<h3 id="list-products-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Paginated products|ProductPage|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|Error|

<aside class="success">
This operation does not require authentication
</aside>

## List active storefront brands

<a id="opIdlistBrands"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/brands',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/brands`

<h3 id="list-active-storefront-brands-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Active brands|BrandListResponse|

<aside class="success">
This operation does not require authentication
</aside>

## List storefront product attributes

<a id="opIdlistProductAttributes"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/product-attributes',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/product-attributes`

<h3 id="list-storefront-product-attributes-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Filterable product attribute definitions|ProductAttributeDefinitionListResponse|

<aside class="success">
This operation does not require authentication
</aside>

## Get product by id

<a id="opIdgetProduct"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/products/{id}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/products/{id}`

<h3 id="get-product-by-id-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="get-product-by-id-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Product|Product|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="success">
This operation does not require authentication
</aside>

<h1 id="ecommerce-api-profile">profile</h1>

## getProfile

<a id="opIdgetProfile"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/me/`

<h3 id="getprofile-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Profile|User|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Unauthorized|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateProfile

<a id="opIdupdateProfile"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "currency": "str",
  "profile_photo_url": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/me/`

> Body parameter

```json
{
  "name": "string",
  "currency": "str",
  "profile_photo_url": "string"
}
```

<h3 id="updateprofile-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|UpdateProfileRequest|true|none|

<h3 id="updateprofile-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated profile|User|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listSavedPaymentMethods

<a id="opIdlistSavedPaymentMethods"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/payment-methods',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/me/payment-methods`

<h3 id="listsavedpaymentmethods-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Saved payment methods|Inline|

<h3 id="listsavedpaymentmethods-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[SavedPaymentMethod]|false|none|none|
|» id|integer|true|none|none|
|» user_id|integer|true|none|none|
|» type|string|true|none|none|
|» brand|string|true|none|none|
|» last4|string|true|none|none|
|» exp_month|integer|true|none|none|
|» exp_year|integer|true|none|none|
|» cardholder_name|string|true|none|none|
|» nickname|string|true|none|none|
|» is_default|boolean|true|none|none|
|» created_at|string(date-time)|true|none|none|
|» updated_at|string(date-time)|true|none|none|
|» deleted_at|string(date-time)¦null|false|none|none|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createSavedPaymentMethod

<a id="opIdcreateSavedPaymentMethod"></a>

> Code samples

```javascript
const inputBody = '{
  "cardholder_name": "string",
  "card_number": "string",
  "exp_month": 1,
  "exp_year": 0,
  "nickname": "string",
  "set_default": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/payment-methods',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/me/payment-methods`

> Body parameter

```json
{
  "cardholder_name": "string",
  "card_number": "string",
  "exp_month": 1,
  "exp_year": 0,
  "nickname": "string",
  "set_default": true
}
```

<h3 id="createsavedpaymentmethod-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CreateSavedPaymentMethodRequest|true|none|

<h3 id="createsavedpaymentmethod-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created payment method|SavedPaymentMethod|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteSavedPaymentMethod

<a id="opIddeleteSavedPaymentMethod"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/payment-methods/{id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/me/payment-methods/{id}`

<h3 id="deletesavedpaymentmethod-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="deletesavedpaymentmethod-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted|MessageResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## setDefaultPaymentMethod

<a id="opIdsetDefaultPaymentMethod"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/payment-methods/{id}/default',
{
  method: 'PATCH',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/me/payment-methods/{id}/default`

<h3 id="setdefaultpaymentmethod-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="setdefaultpaymentmethod-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated payment method|SavedPaymentMethod|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listSavedAddresses

<a id="opIdlistSavedAddresses"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/addresses',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/me/addresses`

<h3 id="listsavedaddresses-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Saved addresses|Inline|

<h3 id="listsavedaddresses-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[SavedAddress]|false|none|none|
|» id|integer|true|none|none|
|» user_id|integer|true|none|none|
|» label|string|true|none|none|
|» full_name|string|true|none|none|
|» line1|string|true|none|none|
|» line2|string|true|none|none|
|» city|string|true|none|none|
|» state|string|true|none|none|
|» postal_code|string|true|none|none|
|» country|string|true|none|none|
|» phone|string|true|none|none|
|» is_default|boolean|true|none|none|
|» created_at|string(date-time)|true|none|none|
|» updated_at|string(date-time)|true|none|none|
|» deleted_at|string(date-time)¦null|false|none|none|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createSavedAddress

<a id="opIdcreateSavedAddress"></a>

> Code samples

```javascript
const inputBody = '{
  "label": "string",
  "full_name": "string",
  "line1": "string",
  "line2": "string",
  "city": "string",
  "state": "string",
  "postal_code": "string",
  "country": "string",
  "phone": "string",
  "set_default": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/addresses',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/me/addresses`

> Body parameter

```json
{
  "label": "string",
  "full_name": "string",
  "line1": "string",
  "line2": "string",
  "city": "string",
  "state": "string",
  "postal_code": "string",
  "country": "string",
  "phone": "string",
  "set_default": true
}
```

<h3 id="createsavedaddress-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CreateSavedAddressRequest|true|none|

<h3 id="createsavedaddress-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created address|SavedAddress|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteSavedAddress

<a id="opIddeleteSavedAddress"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/addresses/{id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/me/addresses/{id}`

<h3 id="deletesavedaddress-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="deletesavedaddress-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted|MessageResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## setDefaultAddress

<a id="opIdsetDefaultAddress"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/addresses/{id}/default',
{
  method: 'PATCH',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/me/addresses/{id}/default`

<h3 id="setdefaultaddress-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="setdefaultaddress-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated address|SavedAddress|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

<h1 id="ecommerce-api-cart">cart</h1>

## getCart

<a id="opIdgetCart"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/cart',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/me/cart`

<h3 id="getcart-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Cart|Cart|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## addCartItem

<a id="opIdaddCartItem"></a>

> Code samples

```javascript
const inputBody = '{
  "product_variant_id": 1,
  "quantity": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/cart',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/me/cart`

> Body parameter

```json
{
  "product_variant_id": 1,
  "quantity": 1
}
```

<h3 id="addcartitem-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|AddCartItemRequest|true|none|

<h3 id="addcartitem-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Cart|Cart|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateCartItem

<a id="opIdupdateCartItem"></a>

> Code samples

```javascript
const inputBody = '{
  "quantity": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/cart/{itemId}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/me/cart/{itemId}`

> Body parameter

```json
{
  "quantity": 1
}
```

<h3 id="updatecartitem-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|itemId|path|integer|true|none|
|body|body|UpdateCartItemRequest|true|none|

<h3 id="updatecartitem-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated item|CartItem|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteCartItem

<a id="opIddeleteCartItem"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/cart/{itemId}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/me/cart/{itemId}`

<h3 id="deletecartitem-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|itemId|path|integer|true|none|

<h3 id="deletecartitem-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

<h1 id="ecommerce-api-checkout">checkout</h1>

## getCheckoutCart

<a id="opIdgetCheckoutCart"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/checkout/cart',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/checkout/cart`

<h3 id="getcheckoutcart-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Cart|Cart|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getCheckoutCartSummary

<a id="opIdgetCheckoutCartSummary"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/checkout/cart/summary',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/checkout/cart/summary`

<h3 id="getcheckoutcartsummary-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Cart summary|CheckoutCartSummary|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## addCheckoutCartItem

<a id="opIdaddCheckoutCartItem"></a>

> Code samples

```javascript
const inputBody = '{
  "product_variant_id": 1,
  "quantity": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/checkout/cart/items',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/checkout/cart/items`

> Body parameter

```json
{
  "product_variant_id": 1,
  "quantity": 1
}
```

<h3 id="addcheckoutcartitem-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|AddCartItemRequest|true|none|

<h3 id="addcheckoutcartitem-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Cart|Cart|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateCheckoutCartItem

<a id="opIdupdateCheckoutCartItem"></a>

> Code samples

```javascript
const inputBody = '{
  "quantity": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/checkout/cart/items/{itemId}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/checkout/cart/items/{itemId}`

> Body parameter

```json
{
  "quantity": 1
}
```

<h3 id="updatecheckoutcartitem-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|itemId|path|integer|true|none|
|body|body|UpdateCartItemRequest|true|none|

<h3 id="updatecheckoutcartitem-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated item|CartItem|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteCheckoutCartItem

<a id="opIddeleteCheckoutCartItem"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/checkout/cart/items/{itemId}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/checkout/cart/items/{itemId}`

<h3 id="deletecheckoutcartitem-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|itemId|path|integer|true|none|

<h3 id="deletecheckoutcartitem-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted|MessageResponse|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listCheckoutPlugins

<a id="opIdlistCheckoutPlugins"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/checkout/plugins',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/me/checkout/plugins`

<h3 id="listcheckoutplugins-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Available checkout provider plugins|CheckoutPluginCatalog|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listCheckoutSessionPlugins

<a id="opIdlistCheckoutSessionPlugins"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/checkout/plugins',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/checkout/plugins`

<h3 id="listcheckoutsessionplugins-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Available checkout provider plugins|CheckoutPluginCatalog|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## quoteCheckout

<a id="opIdquoteCheckout"></a>

> Code samples

```javascript
const inputBody = '{
  "payment_provider_id": "string",
  "shipping_provider_id": "string",
  "tax_provider_id": "string",
  "payment_data": {
    "property1": "string",
    "property2": "string"
  },
  "shipping_data": {
    "property1": "string",
    "property2": "string"
  },
  "tax_data": {
    "property1": "string",
    "property2": "string"
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/checkout/quote',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/me/checkout/quote`

> Body parameter

```json
{
  "payment_provider_id": "string",
  "shipping_provider_id": "string",
  "tax_provider_id": "string",
  "payment_data": {
    "property1": "string",
    "property2": "string"
  },
  "shipping_data": {
    "property1": "string",
    "property2": "string"
  },
  "tax_data": {
    "property1": "string",
    "property2": "string"
  }
}
```

<h3 id="quotecheckout-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CheckoutQuoteRequest|true|none|

<h3 id="quotecheckout-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Quote|CheckoutQuoteResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request payload|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## quoteCheckoutSession

<a id="opIdquoteCheckoutSession"></a>

> Code samples

```javascript
const inputBody = '{
  "payment_provider_id": "string",
  "shipping_provider_id": "string",
  "tax_provider_id": "string",
  "payment_data": {
    "property1": "string",
    "property2": "string"
  },
  "shipping_data": {
    "property1": "string",
    "property2": "string"
  },
  "tax_data": {
    "property1": "string",
    "property2": "string"
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/checkout/quote',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/checkout/quote`

> Body parameter

```json
{
  "payment_provider_id": "string",
  "shipping_provider_id": "string",
  "tax_provider_id": "string",
  "payment_data": {
    "property1": "string",
    "property2": "string"
  },
  "shipping_data": {
    "property1": "string",
    "property2": "string"
  },
  "tax_data": {
    "property1": "string",
    "property2": "string"
  }
}
```

<h3 id="quotecheckoutsession-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CheckoutQuoteRequest|true|none|

<h3 id="quotecheckoutsession-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Quote|CheckoutQuoteResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request payload|Error|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createCheckoutOrder

<a id="opIdcreateCheckoutOrder"></a>

> Code samples

```javascript
const inputBody = '{
  "guest_email": "user@example.com"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Idempotency-Key':'string'
};

fetch('http://localhost:3000/api/v1/checkout/orders',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/checkout/orders`

> Body parameter

```json
{
  "guest_email": "user@example.com"
}
```

<h3 id="createcheckoutorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|Idempotency-Key|header|string|false|none|
|body|body|CreateCheckoutOrderRequest|true|none|

<h3 id="createcheckoutorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created order|Order|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Idempotency conflict|Error|
|429|[Too Many Requests](https://tools.ietf.org/html/rfc6585#section-4)|Checkout submission rate limited|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## authorizeCheckoutOrderPayment

<a id="opIdauthorizeCheckoutOrderPayment"></a>

> Code samples

```javascript
const inputBody = '{
  "snapshot_id": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Idempotency-Key':'string'
};

fetch('http://localhost:3000/api/v1/checkout/orders/{id}/payments/authorize',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/checkout/orders/{id}/payments/authorize`

> Body parameter

```json
{
  "snapshot_id": 1
}
```

<h3 id="authorizecheckoutorderpayment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|Idempotency-Key|header|string|false|none|
|body|body|AuthorizeCheckoutOrderPaymentRequest|false|none|

<h3 id="authorizecheckoutorderpayment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Payment result|ProcessPaymentResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Guest checkout disabled|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Order not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Idempotency conflict|Error|
|429|[Too Many Requests](https://tools.ietf.org/html/rfc6585#section-4)|Checkout submission rate limited|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## quoteCheckoutOrderShippingRates

<a id="opIdquoteCheckoutOrderShippingRates"></a>

> Code samples

```javascript
const inputBody = '{
  "snapshot_id": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Idempotency-Key':'string'
};

fetch('http://localhost:3000/api/v1/checkout/orders/{id}/shipping/rates',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/checkout/orders/{id}/shipping/rates`

> Body parameter

```json
{
  "snapshot_id": 1
}
```

<h3 id="quotecheckoutordershippingrates-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|Idempotency-Key|header|string|false|none|
|body|body|CheckoutOrderShippingRatesRequest|true|none|

<h3 id="quotecheckoutordershippingrates-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Persisted shipping rates for the checkout snapshot|CheckoutOrderShippingRatesResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Order not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Snapshot mismatch|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getCheckoutOrderShippingTracking

<a id="opIdgetCheckoutOrderShippingTracking"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/checkout/orders/{id}/shipping/tracking',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/checkout/orders/{id}/shipping/tracking`

<h3 id="getcheckoutordershippingtracking-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getcheckoutordershippingtracking-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Shipment tracking timeline for the order|CheckoutOrderTrackingResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Order not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## finalizeCheckoutOrderTax

<a id="opIdfinalizeCheckoutOrderTax"></a>

> Code samples

```javascript
const inputBody = '{
  "snapshot_id": 1,
  "inclusive_pricing": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Idempotency-Key':'string'
};

fetch('http://localhost:3000/api/v1/checkout/orders/{id}/tax/finalize',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/checkout/orders/{id}/tax/finalize`

> Body parameter

```json
{
  "snapshot_id": 1,
  "inclusive_pricing": true
}
```

<h3 id="finalizecheckoutordertax-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|Idempotency-Key|header|string|false|none|
|body|body|CheckoutOrderTaxFinalizeRequest|true|none|

<h3 id="finalizecheckoutordertax-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Tax finalization result|CheckoutOrderTaxFinalizeResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Order not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Snapshot mismatch|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

<h1 id="ecommerce-api-orders">orders</h1>

## listUserOrders

<a id="opIdlistUserOrders"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/orders',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/me/orders`

<h3 id="listuserorders-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|status|query|string|false|none|
|start_date|query|string|false|none|
|end_date|query|string|false|none|
|page|query|integer|false|none|
|limit|query|integer|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|PENDING|
|status|PAID|
|status|FAILED|
|status|SHIPPED|
|status|DELIVERED|
|status|CANCELLED|
|status|REFUNDED|

<h3 id="listuserorders-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Orders page|OrderPage|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createOrder

<a id="opIdcreateOrder"></a>

> Code samples

```javascript
const inputBody = '{
  "items": [
    {
      "product_variant_id": 1,
      "quantity": 1
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/orders',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/me/orders`

> Body parameter

```json
{
  "items": [
    {
      "product_variant_id": 1,
      "quantity": 1
    }
  ]
}
```

<h3 id="createorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CreateOrderRequest|true|none|

<h3 id="createorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created order|Order|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## claimGuestOrder

<a id="opIdclaimGuestOrder"></a>

> Code samples

```javascript
const inputBody = '{
  "email": "user@example.com",
  "confirmation_token": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/orders/claim',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/me/orders/claim`

> Body parameter

```json
{
  "email": "user@example.com",
  "confirmation_token": "string"
}
```

<h3 id="claimguestorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|ClaimGuestOrderRequest|true|none|

<h3 id="claimguestorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Guest order linked to authenticated user|ClaimGuestOrderResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Guest order not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Guest order already claimed|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getUserOrder

<a id="opIdgetUserOrder"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/orders/{id}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/me/orders/{id}`

<h3 id="getuserorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getuserorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Order|Order|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## cancelUserOrder

<a id="opIdcancelUserOrder"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/orders/{id}/cancel',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/me/orders/{id}/cancel`

<h3 id="canceluserorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="canceluserorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Cancelled order|Order|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

<h1 id="ecommerce-api-admin">admin</h1>

## listAdminProducts

<a id="opIdlistAdminProducts"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/products`

<h3 id="listadminproducts-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|q|query|string|false|none|
|min_price|query|number(double)|false|none|
|max_price|query|number(double)|false|none|
|brand_slug|query|string|false|none|
|category_slug|query|array[string]|false|none|
|category_id|query|array[integer]|false|none|
|include_inactive_categories|query|boolean|false|none|
|has_variant_stock|query|boolean|false|none|
|attribute|query|object|false|none|
|sort|query|string|false|none|
|order|query|string|false|none|
|page|query|integer|false|none|
|limit|query|integer|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|sort|price|
|sort|name|
|sort|created_at|
|order|asc|
|order|desc|

<h3 id="listadminproducts-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Paginated admin products|ProductPage|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createProduct

<a id="opIdcreateProduct"></a>

> Code samples

```javascript
const inputBody = '{
  "sku": "string",
  "name": "string",
  "subtitle": "string",
  "description": "string",
  "images": [
    "string"
  ],
  "related_product_ids": [
    1
  ],
  "category_ids": [
    1
  ],
  "brand_id": 1,
  "default_variant_sku": "string",
  "options": [
    {
      "name": "string",
      "position": 1,
      "display_type": "string",
      "values": [
        {
          "value": "string",
          "position": 1
        }
      ]
    }
  ],
  "variants": [
    {
      "sku": "string",
      "title": "string",
      "price": 0.1,
      "compare_at_price": 0.1,
      "stock": 0,
      "position": 1,
      "is_published": true,
      "weight_grams": 0,
      "length_cm": 0.1,
      "width_cm": 0.1,
      "height_cm": 0.1,
      "selections": [
        {
          "option_name": "string",
          "option_value": "string",
          "position": 1
        }
      ]
    }
  ],
  "attributes": [
    {
      "product_attribute_id": 1,
      "text_value": "string",
      "number_value": 0.1,
      "boolean_value": true,
      "enum_value": "string",
      "position": 1
    }
  ],
  "seo": {
    "title": "string",
    "description": "string",
    "canonical_path": "string",
    "og_image_media_id": "string",
    "noindex": true
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/products`

> Body parameter

```json
{
  "sku": "string",
  "name": "string",
  "subtitle": "string",
  "description": "string",
  "images": [
    "string"
  ],
  "related_product_ids": [
    1
  ],
  "category_ids": [
    1
  ],
  "brand_id": 1,
  "default_variant_sku": "string",
  "options": [
    {
      "name": "string",
      "position": 1,
      "display_type": "string",
      "values": [
        {
          "value": "string",
          "position": 1
        }
      ]
    }
  ],
  "variants": [
    {
      "sku": "string",
      "title": "string",
      "price": 0.1,
      "compare_at_price": 0.1,
      "stock": 0,
      "position": 1,
      "is_published": true,
      "weight_grams": 0,
      "length_cm": 0.1,
      "width_cm": 0.1,
      "height_cm": 0.1,
      "selections": [
        {
          "option_name": "string",
          "option_value": "string",
          "position": 1
        }
      ]
    }
  ],
  "attributes": [
    {
      "product_attribute_id": 1,
      "text_value": "string",
      "number_value": 0.1,
      "boolean_value": true,
      "enum_value": "string",
      "position": 1
    }
  ],
  "seo": {
    "title": "string",
    "description": "string",
    "canonical_path": "string",
    "og_image_media_id": "string",
    "noindex": true
  }
}
```

<h3 id="createproduct-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|ProductUpsertInput|true|none|

<h3 id="createproduct-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created product|Product|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminDiscountCampaigns

<a id="opIdlistAdminDiscountCampaigns"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/campaigns',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/discounts/campaigns`

<h3 id="listadmindiscountcampaigns-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|status|query|string|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|active|
|status|scheduled|
|status|disabled|
|status|archived|

<h3 id="listadmindiscountcampaigns-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Product discount campaigns|DiscountCampaignListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminDiscountCampaign

<a id="opIdcreateAdminDiscountCampaign"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "product_ids": [
    1
  ],
  "discount_mode": "percent",
  "discount_value": 0.01,
  "starts_at": "2019-08-24T14:15:22Z",
  "ends_at": "2019-08-24T14:15:22Z",
  "priority": 0,
  "is_exclusive": false,
  "status": "active",
  "metadata": {},
  "coupon_code": "string",
  "channels": [
    "web"
  ],
  "customer_segment": "string",
  "global_usage_cap": 1,
  "per_customer_usage_cap": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/campaigns',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/campaigns`

> Body parameter

```json
{
  "name": "string",
  "product_ids": [
    1
  ],
  "discount_mode": "percent",
  "discount_value": 0.01,
  "starts_at": "2019-08-24T14:15:22Z",
  "ends_at": "2019-08-24T14:15:22Z",
  "priority": 0,
  "is_exclusive": false,
  "status": "active",
  "metadata": {},
  "coupon_code": "string",
  "channels": [
    "web"
  ],
  "customer_segment": "string",
  "global_usage_cap": 1,
  "per_customer_usage_cap": 1
}
```

<h3 id="createadmindiscountcampaign-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|ProductDiscountInput|true|none|

<h3 id="createadmindiscountcampaign-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created product discount campaign|DiscountCampaign|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid discount campaign|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminPromotionCampaign

<a id="opIdcreateAdminPromotionCampaign"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "starts_at": "2019-08-24T14:15:22Z",
  "ends_at": "2019-08-24T14:15:22Z",
  "priority": 0,
  "is_exclusive": false,
  "status": "active",
  "metadata": {},
  "coupon_code": "string",
  "channels": [
    "web"
  ],
  "customer_segment": "string",
  "global_usage_cap": 1,
  "per_customer_usage_cap": 1,
  "rules": [
    {
      "condition": {
        "product_ids": [
          1
        ],
        "product_variant_ids": [
          1
        ],
        "category_ids": [
          1
        ],
        "brand_ids": [
          1
        ],
        "min_quantity": 1,
        "min_subtotal": 0.01
      },
      "action": {
        "mode": "percent",
        "value": 0.1,
        "target_type": "cart",
        "target_ids": [
          1
        ],
        "product_ids": [
          1
        ],
        "product_variant_ids": [
          1
        ],
        "category_ids": [
          1
        ],
        "brand_ids": [
          1
        ],
        "sku": "string"
      },
      "stack_policy": "none",
      "max_applications_per_order": 1
    }
  ],
  "levels": [
    {
      "name": "string",
      "priority": 0,
      "action": {
        "mode": "percent",
        "value": 0.1,
        "target_type": "cart",
        "target_ids": [
          1
        ],
        "product_ids": [
          1
        ],
        "product_variant_ids": [
          1
        ],
        "category_ids": [
          1
        ],
        "brand_ids": [
          1
        ],
        "sku": "string"
      },
      "stack_policy": "none",
      "max_applications_per_order": 1,
      "targets": [
        {
          "target_type": "product",
          "target_id": 1
        }
      ]
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/promotions',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/promotions`

> Body parameter

```json
{
  "name": "string",
  "starts_at": "2019-08-24T14:15:22Z",
  "ends_at": "2019-08-24T14:15:22Z",
  "priority": 0,
  "is_exclusive": false,
  "status": "active",
  "metadata": {},
  "coupon_code": "string",
  "channels": [
    "web"
  ],
  "customer_segment": "string",
  "global_usage_cap": 1,
  "per_customer_usage_cap": 1,
  "rules": [
    {
      "condition": {
        "product_ids": [
          1
        ],
        "product_variant_ids": [
          1
        ],
        "category_ids": [
          1
        ],
        "brand_ids": [
          1
        ],
        "min_quantity": 1,
        "min_subtotal": 0.01
      },
      "action": {
        "mode": "percent",
        "value": 0.1,
        "target_type": "cart",
        "target_ids": [
          1
        ],
        "product_ids": [
          1
        ],
        "product_variant_ids": [
          1
        ],
        "category_ids": [
          1
        ],
        "brand_ids": [
          1
        ],
        "sku": "string"
      },
      "stack_policy": "none",
      "max_applications_per_order": 1
    }
  ],
  "levels": [
    {
      "name": "string",
      "priority": 0,
      "action": {
        "mode": "percent",
        "value": 0.1,
        "target_type": "cart",
        "target_ids": [
          1
        ],
        "product_ids": [
          1
        ],
        "product_variant_ids": [
          1
        ],
        "category_ids": [
          1
        ],
        "brand_ids": [
          1
        ],
        "sku": "string"
      },
      "stack_policy": "none",
      "max_applications_per_order": 1,
      "targets": [
        {
          "target_type": "product",
          "target_id": 1
        }
      ]
    }
  ]
}
```

<h3 id="createadminpromotioncampaign-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|PromotionInput|true|none|

<h3 id="createadminpromotioncampaign-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created promotion campaign|DiscountCampaign|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid promotion|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## previewAdminPromotion

<a id="opIdpreviewAdminPromotion"></a>

> Code samples

```javascript
const inputBody = '{
  "coupon_code": "string",
  "channel": "web",
  "customer_segment": "string",
  "lines": [
    {
      "product_id": 1,
      "product_variant_id": 1,
      "brand_id": 1,
      "category_ids": [
        1
      ],
      "sku": "string",
      "quantity": 1,
      "unit_price": 0.1
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/promotions/preview',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/promotions/preview`

> Body parameter

```json
{
  "coupon_code": "string",
  "channel": "web",
  "customer_segment": "string",
  "lines": [
    {
      "product_id": 1,
      "product_variant_id": 1,
      "brand_id": 1,
      "category_ids": [
        1
      ],
      "sku": "string",
      "quantity": 1,
      "unit_price": 0.1
    }
  ]
}
```

<h3 id="previewadminpromotion-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|PromotionEvaluationRequest|true|none|

<h3 id="previewadminpromotion-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Promotion evaluation preview|PromotionEvaluationResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid preview request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminPromotionTemplates

<a id="opIdlistAdminPromotionTemplates"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/templates',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/discounts/templates`

<h3 id="listadminpromotiontemplates-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|active|query|boolean|false|none|

<h3 id="listadminpromotiontemplates-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Promotion templates|PromotionTemplateListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminPromotionTemplate

<a id="opIdcreateAdminPromotionTemplate"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "description": "string",
  "template": {
    "name": "string",
    "starts_at": "2019-08-24T14:15:22Z",
    "ends_at": "2019-08-24T14:15:22Z",
    "priority": 0,
    "is_exclusive": false,
    "status": "active",
    "metadata": {},
    "coupon_code": "string",
    "channels": [
      "web"
    ],
    "customer_segment": "string",
    "global_usage_cap": 1,
    "per_customer_usage_cap": 1,
    "rules": [
      {
        "condition": {
          "product_ids": [
            1
          ],
          "product_variant_ids": [
            1
          ],
          "category_ids": [
            1
          ],
          "brand_ids": [
            1
          ],
          "min_quantity": 1,
          "min_subtotal": 0.01
        },
        "action": {
          "mode": "percent",
          "value": 0.1,
          "target_type": "cart",
          "target_ids": [
            1
          ],
          "product_ids": [
            1
          ],
          "product_variant_ids": [
            1
          ],
          "category_ids": [
            1
          ],
          "brand_ids": [
            1
          ],
          "sku": "string"
        },
        "stack_policy": "none",
        "max_applications_per_order": 1
      }
    ],
    "levels": [
      {
        "name": "string",
        "priority": 0,
        "action": {
          "mode": "percent",
          "value": 0.1,
          "target_type": "cart",
          "target_ids": [
            1
          ],
          "product_ids": [
            1
          ],
          "product_variant_ids": [
            1
          ],
          "category_ids": [
            1
          ],
          "brand_ids": [
            1
          ],
          "sku": "string"
        },
        "stack_policy": "none",
        "max_applications_per_order": 1,
        "targets": [
          {
            "target_type": "product",
            "target_id": 1
          }
        ]
      }
    ]
  },
  "is_active": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/templates',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/templates`

> Body parameter

```json
{
  "name": "string",
  "description": "string",
  "template": {
    "name": "string",
    "starts_at": "2019-08-24T14:15:22Z",
    "ends_at": "2019-08-24T14:15:22Z",
    "priority": 0,
    "is_exclusive": false,
    "status": "active",
    "metadata": {},
    "coupon_code": "string",
    "channels": [
      "web"
    ],
    "customer_segment": "string",
    "global_usage_cap": 1,
    "per_customer_usage_cap": 1,
    "rules": [
      {
        "condition": {
          "product_ids": [
            1
          ],
          "product_variant_ids": [
            1
          ],
          "category_ids": [
            1
          ],
          "brand_ids": [
            1
          ],
          "min_quantity": 1,
          "min_subtotal": 0.01
        },
        "action": {
          "mode": "percent",
          "value": 0.1,
          "target_type": "cart",
          "target_ids": [
            1
          ],
          "product_ids": [
            1
          ],
          "product_variant_ids": [
            1
          ],
          "category_ids": [
            1
          ],
          "brand_ids": [
            1
          ],
          "sku": "string"
        },
        "stack_policy": "none",
        "max_applications_per_order": 1
      }
    ],
    "levels": [
      {
        "name": "string",
        "priority": 0,
        "action": {
          "mode": "percent",
          "value": 0.1,
          "target_type": "cart",
          "target_ids": [
            1
          ],
          "product_ids": [
            1
          ],
          "product_variant_ids": [
            1
          ],
          "category_ids": [
            1
          ],
          "brand_ids": [
            1
          ],
          "sku": "string"
        },
        "stack_policy": "none",
        "max_applications_per_order": 1,
        "targets": [
          {
            "target_type": "product",
            "target_id": 1
          }
        ]
      }
    ]
  },
  "is_active": true
}
```

<h3 id="createadminpromotiontemplate-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|PromotionTemplateInput|true|none|

<h3 id="createadminpromotiontemplate-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created promotion template|PromotionTemplate|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid template|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## instantiateAdminPromotionTemplate

<a id="opIdinstantiateAdminPromotionTemplate"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "starts_at": "2019-08-24T14:15:22Z",
  "ends_at": "2019-08-24T14:15:22Z",
  "coupon_code": "string",
  "channels": [
    "web"
  ],
  "customer_segment": "string",
  "global_usage_cap": 1,
  "per_customer_usage_cap": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/templates/{id}/instantiate',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/templates/{id}/instantiate`

> Body parameter

```json
{
  "name": "string",
  "starts_at": "2019-08-24T14:15:22Z",
  "ends_at": "2019-08-24T14:15:22Z",
  "coupon_code": "string",
  "channels": [
    "web"
  ],
  "customer_segment": "string",
  "global_usage_cap": 1,
  "per_customer_usage_cap": 1
}
```

<h3 id="instantiateadminpromotiontemplate-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|PromotionTemplateInstantiateInput|true|none|

<h3 id="instantiateadminpromotiontemplate-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created promotion campaign|DiscountCampaign|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid template instantiation|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Promotion template not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminDiscountCampaign

<a id="opIdupdateAdminDiscountCampaign"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "product_ids": [
    1
  ],
  "discount_mode": "percent",
  "discount_value": 0.01,
  "starts_at": "2019-08-24T14:15:22Z",
  "ends_at": "2019-08-24T14:15:22Z",
  "priority": 0,
  "is_exclusive": false,
  "status": "active",
  "metadata": {},
  "coupon_code": "string",
  "channels": [
    "web"
  ],
  "customer_segment": "string",
  "global_usage_cap": 1,
  "per_customer_usage_cap": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/campaigns/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/discounts/campaigns/{id}`

> Body parameter

```json
{
  "name": "string",
  "product_ids": [
    1
  ],
  "discount_mode": "percent",
  "discount_value": 0.01,
  "starts_at": "2019-08-24T14:15:22Z",
  "ends_at": "2019-08-24T14:15:22Z",
  "priority": 0,
  "is_exclusive": false,
  "status": "active",
  "metadata": {},
  "coupon_code": "string",
  "channels": [
    "web"
  ],
  "customer_segment": "string",
  "global_usage_cap": 1,
  "per_customer_usage_cap": 1
}
```

<h3 id="updateadmindiscountcampaign-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|ProductDiscountInput|true|none|

<h3 id="updateadmindiscountcampaign-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated product discount campaign|DiscountCampaign|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid discount campaign|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Discount campaign not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## disableAdminDiscountCampaign

<a id="opIddisableAdminDiscountCampaign"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/campaigns/{id}/disable',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/campaigns/{id}/disable`

<h3 id="disableadmindiscountcampaign-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="disableadmindiscountcampaign-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Disabled product discount campaign|DiscountCampaign|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Discount campaign not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## scheduleAdminDiscountCampaign

<a id="opIdscheduleAdminDiscountCampaign"></a>

> Code samples

```javascript
const inputBody = '{
  "schedule_type": "one_time",
  "recurrence": "daily",
  "window_start": "2019-08-24T14:15:22Z",
  "window_end": "2019-08-24T14:15:22Z",
  "until_at": "2019-08-24T14:15:22Z",
  "timezone": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/campaigns/{id}/schedule',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/campaigns/{id}/schedule`

> Body parameter

```json
{
  "schedule_type": "one_time",
  "recurrence": "daily",
  "window_start": "2019-08-24T14:15:22Z",
  "window_end": "2019-08-24T14:15:22Z",
  "until_at": "2019-08-24T14:15:22Z",
  "timezone": "string"
}
```

<h3 id="scheduleadmindiscountcampaign-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|DiscountScheduleInput|true|none|

<h3 id="scheduleadmindiscountcampaign-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Saved discount schedule|DiscountSchedule|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid schedule|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Discount campaign not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## archiveAdminDiscountCampaign

<a id="opIdarchiveAdminDiscountCampaign"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/campaigns/{id}/archive',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/campaigns/{id}/archive`

<h3 id="archiveadmindiscountcampaign-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="archiveadmindiscountcampaign-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Archived discount campaign|DiscountCampaign|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Discount campaign not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## runAdminDiscountLifecycle

<a id="opIdrunAdminDiscountLifecycle"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/lifecycle/run',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/lifecycle/run`

<h3 id="runadmindiscountlifecycle-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Discount lifecycle run summary|DiscountLifecycleRunResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminDiscountHistory

<a id="opIdlistAdminDiscountHistory"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/history',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/discounts/history`

<h3 id="listadmindiscounthistory-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|campaign_id|query|integer|false|none|

<h3 id="listadmindiscounthistory-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Discount lifecycle history|DiscountStateHistoryListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminDiscountAudit

<a id="opIdlistAdminDiscountAudit"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/audit',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/discounts/audit`

<h3 id="listadmindiscountaudit-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|campaign_id|query|integer|false|none|

<h3 id="listadmindiscountaudit-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Discount campaign audit entries|DiscountCampaignAuditListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminDiscountMetrics

<a id="opIdgetAdminDiscountMetrics"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/metrics',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/discounts/metrics`

<h3 id="getadmindiscountmetrics-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Discount evaluation metrics snapshot|DiscountEvaluationMetrics|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## runAdminDiscountReconciliation

<a id="opIdrunAdminDiscountReconciliation"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/discounts/reconciliation/run',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/discounts/reconciliation/run`

<h3 id="runadmindiscountreconciliation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Discount schedule reconciliation report|DiscountReconciliationReport|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminBrands

<a id="opIdlistAdminBrands"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/brands',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/brands`

<h3 id="listadminbrands-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|q|query|string|false|none|

<h3 id="listadminbrands-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Available brands|BrandListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminBrand

<a id="opIdcreateAdminBrand"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "slug": "string",
  "description": "string",
  "logo_media_id": "string",
  "is_active": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/brands',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/brands`

> Body parameter

```json
{
  "name": "string",
  "slug": "string",
  "description": "string",
  "logo_media_id": "string",
  "is_active": true
}
```

<h3 id="createadminbrand-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|BrandInput|true|none|

<h3 id="createadminbrand-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created brand|Brand|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminBrand

<a id="opIdupdateAdminBrand"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "slug": "string",
  "description": "string",
  "logo_media_id": "string",
  "is_active": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/brands/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/brands/{id}`

> Body parameter

```json
{
  "name": "string",
  "slug": "string",
  "description": "string",
  "logo_media_id": "string",
  "is_active": true
}
```

<h3 id="updateadminbrand-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|BrandInput|true|none|

<h3 id="updateadminbrand-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated brand|Brand|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteAdminBrand

<a id="opIddeleteAdminBrand"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/brands/{id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/brands/{id}`

<h3 id="deleteadminbrand-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="deleteadminbrand-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted brand|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCategories

<a id="opIdlistAdminCategories"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/categories',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/categories`

<h3 id="listadmincategories-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|q|query|string|false|none|
|include_inactive|query|boolean|false|none|

<h3 id="listadmincategories-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Available categories|CategoryListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminCategory

<a id="opIdcreateAdminCategory"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "slug": "string",
  "description": "string",
  "is_active": true,
  "sort_order": 0,
  "parent_id": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/categories',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/categories`

> Body parameter

```json
{
  "name": "string",
  "slug": "string",
  "description": "string",
  "is_active": true,
  "sort_order": 0,
  "parent_id": 1
}
```

<h3 id="createadmincategory-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CategoryInput|true|none|

<h3 id="createadmincategory-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created category|Category|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCategory

<a id="opIdupdateAdminCategory"></a>

> Code samples

```javascript
const inputBody = '{
  "name": "string",
  "slug": "string",
  "description": "string",
  "is_active": true,
  "sort_order": 0,
  "parent_id": 1
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/categories/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/categories/{id}`

> Body parameter

```json
{
  "name": "string",
  "slug": "string",
  "description": "string",
  "is_active": true,
  "sort_order": 0,
  "parent_id": 1
}
```

<h3 id="updateadmincategory-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CategoryInput|true|none|

<h3 id="updateadmincategory-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated category|Category|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteAdminCategory

<a id="opIddeleteAdminCategory"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/categories/{id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/categories/{id}`

<h3 id="deleteadmincategory-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="deleteadmincategory-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted category|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminProductAttributes

<a id="opIdlistAdminProductAttributes"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/product-attributes',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/product-attributes`

<h3 id="listadminproductattributes-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Product attribute definitions|ProductAttributeDefinitionListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminProductAttribute

<a id="opIdcreateAdminProductAttribute"></a>

> Code samples

```javascript
const inputBody = '{
  "key": "string",
  "slug": "string",
  "type": "text",
  "filterable": true,
  "sortable": true,
  "enum_values": [
    "string"
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/product-attributes',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/product-attributes`

> Body parameter

```json
{
  "key": "string",
  "slug": "string",
  "type": "text",
  "filterable": true,
  "sortable": true,
  "enum_values": [
    "string"
  ]
}
```

<h3 id="createadminproductattribute-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|ProductAttributeDefinitionInput|true|none|

<h3 id="createadminproductattribute-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created product attribute definition|ProductAttributeDefinition|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminProductAttribute

<a id="opIdupdateAdminProductAttribute"></a>

> Code samples

```javascript
const inputBody = '{
  "key": "string",
  "slug": "string",
  "type": "text",
  "filterable": true,
  "sortable": true,
  "enum_values": [
    "string"
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/product-attributes/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/product-attributes/{id}`

> Body parameter

```json
{
  "key": "string",
  "slug": "string",
  "type": "text",
  "filterable": true,
  "sortable": true,
  "enum_values": [
    "string"
  ]
}
```

<h3 id="updateadminproductattribute-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|ProductAttributeDefinitionInput|true|none|

<h3 id="updateadminproductattribute-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated product attribute definition|ProductAttributeDefinition|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteAdminProductAttribute

<a id="opIddeleteAdminProductAttribute"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/product-attributes/{id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/product-attributes/{id}`

<h3 id="deleteadminproductattribute-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="deleteadminproductattribute-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted product attribute definition|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminProduct

<a id="opIdgetAdminProduct"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/products/{id}`

<h3 id="getadminproduct-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadminproduct-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Admin product|Product|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateProduct

<a id="opIdupdateProduct"></a>

> Code samples

```javascript
const inputBody = '{
  "sku": "string",
  "name": "string",
  "subtitle": "string",
  "description": "string",
  "images": [
    "string"
  ],
  "related_product_ids": [
    1
  ],
  "category_ids": [
    1
  ],
  "brand_id": 1,
  "default_variant_sku": "string",
  "options": [
    {
      "name": "string",
      "position": 1,
      "display_type": "string",
      "values": [
        {
          "value": "string",
          "position": 1
        }
      ]
    }
  ],
  "variants": [
    {
      "sku": "string",
      "title": "string",
      "price": 0.1,
      "compare_at_price": 0.1,
      "stock": 0,
      "position": 1,
      "is_published": true,
      "weight_grams": 0,
      "length_cm": 0.1,
      "width_cm": 0.1,
      "height_cm": 0.1,
      "selections": [
        {
          "option_name": "string",
          "option_value": "string",
          "position": 1
        }
      ]
    }
  ],
  "attributes": [
    {
      "product_attribute_id": 1,
      "text_value": "string",
      "number_value": 0.1,
      "boolean_value": true,
      "enum_value": "string",
      "position": 1
    }
  ],
  "seo": {
    "title": "string",
    "description": "string",
    "canonical_path": "string",
    "og_image_media_id": "string",
    "noindex": true
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/products/{id}`

> Body parameter

```json
{
  "sku": "string",
  "name": "string",
  "subtitle": "string",
  "description": "string",
  "images": [
    "string"
  ],
  "related_product_ids": [
    1
  ],
  "category_ids": [
    1
  ],
  "brand_id": 1,
  "default_variant_sku": "string",
  "options": [
    {
      "name": "string",
      "position": 1,
      "display_type": "string",
      "values": [
        {
          "value": "string",
          "position": 1
        }
      ]
    }
  ],
  "variants": [
    {
      "sku": "string",
      "title": "string",
      "price": 0.1,
      "compare_at_price": 0.1,
      "stock": 0,
      "position": 1,
      "is_published": true,
      "weight_grams": 0,
      "length_cm": 0.1,
      "width_cm": 0.1,
      "height_cm": 0.1,
      "selections": [
        {
          "option_name": "string",
          "option_value": "string",
          "position": 1
        }
      ]
    }
  ],
  "attributes": [
    {
      "product_attribute_id": 1,
      "text_value": "string",
      "number_value": 0.1,
      "boolean_value": true,
      "enum_value": "string",
      "position": 1
    }
  ],
  "seo": {
    "title": "string",
    "description": "string",
    "canonical_path": "string",
    "og_image_media_id": "string",
    "noindex": true
  }
}
```

<h3 id="updateproduct-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|ProductUpsertInput|true|none|

<h3 id="updateproduct-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated product|Product|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteProduct

<a id="opIddeleteProduct"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/products/{id}`

<h3 id="deleteproduct-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="deleteproduct-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## attachProductMedia

<a id="opIdattachProductMedia"></a>

> Code samples

```javascript
const inputBody = '{
  "media_ids": [
    "string"
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}/media',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/products/{id}/media`

> Body parameter

```json
{
  "media_ids": [
    "string"
  ]
}
```

<h3 id="attachproductmedia-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|MediaIDsRequest|true|none|

<h3 id="attachproductmedia-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated product|Product|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## publishProduct

<a id="opIdpublishProduct"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}/publish',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/products/{id}/publish`

<h3 id="publishproduct-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="publishproduct-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published product|Product|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## unpublishProduct

<a id="opIdunpublishProduct"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}/unpublish',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/products/{id}/unpublish`

<h3 id="unpublishproduct-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="unpublishproduct-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Unpublished product|Product|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## discardProductDraft

<a id="opIddiscardProductDraft"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}/draft',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/products/{id}/draft`

<h3 id="discardproductdraft-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="discardproductdraft-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Product after draft discard|Product|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateProductMediaOrder

<a id="opIdupdateProductMediaOrder"></a>

> Code samples

```javascript
const inputBody = '{
  "media_ids": [
    "string"
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}/media/order',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/products/{id}/media/order`

> Body parameter

```json
{
  "media_ids": [
    "string"
  ]
}
```

<h3 id="updateproductmediaorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|MediaIDsRequest|true|none|

<h3 id="updateproductmediaorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated product|Product|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## detachProductMedia

<a id="opIddetachProductMedia"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}/media/{mediaId}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/products/{id}/media/{mediaId}`

<h3 id="detachproductmedia-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|mediaId|path|string|true|none|

<h3 id="detachproductmedia-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated product|Product|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateProductRelated

<a id="opIdupdateProductRelated"></a>

> Code samples

```javascript
const inputBody = '{
  "related_ids": [
    1
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/products/{id}/related',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/products/{id}/related`

> Body parameter

```json
{
  "related_ids": [
    1
  ]
}
```

<h3 id="updateproductrelated-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|UpdateRelatedRequest|true|none|

<h3 id="updateproductrelated-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated product|Product|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminOrders

<a id="opIdlistAdminOrders"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/orders',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/orders`

<h3 id="listadminorders-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|page|query|integer|false|none|
|limit|query|integer|false|none|
|q|query|string|false|none|

<h3 id="listadminorders-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Orders page|OrderPage|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminOrder

<a id="opIdgetAdminOrder"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/orders/{id}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/orders/{id}`

<h3 id="getadminorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadminorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Order|Order|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminOrderPayments

<a id="opIdgetAdminOrderPayments"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/orders/{id}/payments',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/orders/{id}/payments`

<h3 id="getadminorderpayments-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadminorderpayments-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Order payment ledger|OrderPaymentLedger|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Order not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminOrderShippingLabel

<a id="opIdcreateAdminOrderShippingLabel"></a>

> Code samples

```javascript
const inputBody = '{
  "rate_id": 1,
  "package": {
    "reference": "string",
    "weight_grams": 0,
    "length_cm": 0,
    "width_cm": 0,
    "height_cm": 0
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Idempotency-Key':'string'
};

fetch('http://localhost:3000/api/v1/admin/orders/{id}/shipping/labels',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/orders/{id}/shipping/labels`

> Body parameter

```json
{
  "rate_id": 1,
  "package": {
    "reference": "string",
    "weight_grams": 0,
    "length_cm": 0,
    "width_cm": 0,
    "height_cm": 0
  }
}
```

<h3 id="createadminordershippinglabel-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|Idempotency-Key|header|string|true|none|
|body|body|AdminOrderShippingLabelRequest|true|none|

<h3 id="createadminordershippinglabel-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Shipping label purchased|AdminOrderShippingLabelResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Order or rate not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Shipment already finalized with a different service|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## captureAdminOrderPayment

<a id="opIdcaptureAdminOrderPayment"></a>

> Code samples

```javascript
const inputBody = '{
  "amount": 0.01
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Idempotency-Key':'string'
};

fetch('http://localhost:3000/api/v1/admin/orders/{id}/payments/{intentId}/capture',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/orders/{id}/payments/{intentId}/capture`

> Body parameter

```json
{
  "amount": 0.01
}
```

<h3 id="captureadminorderpayment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|intentId|path|integer|true|none|
|Idempotency-Key|header|string|true|none|
|body|body|AdminOrderPaymentAmountRequest|false|none|

<h3 id="captureadminorderpayment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Capture result|AdminOrderPaymentLifecycleResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## voidAdminOrderPayment

<a id="opIdvoidAdminOrderPayment"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json',
  'Idempotency-Key':'string'
};

fetch('http://localhost:3000/api/v1/admin/orders/{id}/payments/{intentId}/void',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/orders/{id}/payments/{intentId}/void`

<h3 id="voidadminorderpayment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|intentId|path|integer|true|none|
|Idempotency-Key|header|string|true|none|

<h3 id="voidadminorderpayment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Void result|AdminOrderPaymentLifecycleResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## refundAdminOrderPayment

<a id="opIdrefundAdminOrderPayment"></a>

> Code samples

```javascript
const inputBody = '{
  "amount": 0.01
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json',
  'Idempotency-Key':'string'
};

fetch('http://localhost:3000/api/v1/admin/orders/{id}/payments/{intentId}/refund',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/orders/{id}/payments/{intentId}/refund`

> Body parameter

```json
{
  "amount": 0.01
}
```

<h3 id="refundadminorderpayment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|intentId|path|integer|true|none|
|Idempotency-Key|header|string|true|none|
|body|body|AdminOrderPaymentAmountRequest|false|none|

<h3 id="refundadminorderpayment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Refund result|AdminOrderPaymentLifecycleResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateOrderStatus

<a id="opIdupdateOrderStatus"></a>

> Code samples

```javascript
const inputBody = '{
  "status": "PENDING"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/orders/{id}/status',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/orders/{id}/status`

> Body parameter

```json
{
  "status": "PENDING"
}
```

<h3 id="updateorderstatus-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|UpdateOrderStatusRequest|true|none|

<h3 id="updateorderstatus-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated order|Order|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminWebhookEvents

<a id="opIdlistAdminWebhookEvents"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/webhooks/events',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/webhooks/events`

<h3 id="listadminwebhookevents-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|provider|query|string|false|none|
|status|query|string|false|none|
|page|query|integer|false|none|
|limit|query|integer|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|PENDING|
|status|PROCESSED|
|status|DEAD_LETTER|
|status|REJECTED|

<h3 id="listadminwebhookevents-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Webhook event page|WebhookEventPage|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## exportAdminTaxReport

<a id="opIdexportAdminTaxReport"></a>

> Code samples

```javascript

const headers = {
  'Accept':'text/csv'
};

fetch('http://localhost:3000/api/v1/admin/tax/reports/export',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/tax/reports/export`

<h3 id="exportadmintaxreport-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|provider|query|string|false|none|
|start_date|query|string(date-time)|false|none|
|end_date|query|string(date-time)|false|none|
|format|query|string|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|format|csv|

<h3 id="exportadmintaxreport-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Tax export|string|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminProviderCredentials

<a id="opIdlistAdminProviderCredentials"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/providers/credentials',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/providers/credentials`

<h3 id="listadminprovidercredentials-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|provider_type|query|string|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|provider_type|payment|
|provider_type|shipping|
|provider_type|tax|

<h3 id="listadminprovidercredentials-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Provider credential metadata|ProviderCredentialListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## upsertAdminProviderCredential

<a id="opIdupsertAdminProviderCredential"></a>

> Code samples

```javascript
const inputBody = '{
  "provider_type": "payment",
  "provider_id": "string",
  "environment": "sandbox",
  "label": "string",
  "secret_data": {
    "property1": "string",
    "property2": "string"
  },
  "supported_currencies": [
    "string"
  ],
  "settlement_currency": "string",
  "fx_mode": "same_currency_only"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/providers/credentials',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/providers/credentials`

> Body parameter

```json
{
  "provider_type": "payment",
  "provider_id": "string",
  "environment": "sandbox",
  "label": "string",
  "secret_data": {
    "property1": "string",
    "property2": "string"
  },
  "supported_currencies": [
    "string"
  ],
  "settlement_currency": "string",
  "fx_mode": "same_currency_only"
}
```

<h3 id="upsertadminprovidercredential-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|ProviderCredentialRequest|true|none|

<h3 id="upsertadminprovidercredential-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Stored provider credential metadata|ProviderCredentialEnvelope|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|412|[Precondition Failed](https://tools.ietf.org/html/rfc7232#section-4.2)|Credential encryption is not configured|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminProviderOperationsOverview

<a id="opIdgetAdminProviderOperationsOverview"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/providers/overview',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/providers/overview`

<h3 id="getadminprovideroperationsoverview-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Provider operations overview|ProviderOperationsOverview|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## rotateAdminProviderCredential

<a id="opIdrotateAdminProviderCredential"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/providers/credentials/{id}/rotate',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/providers/credentials/{id}/rotate`

<h3 id="rotateadminprovidercredential-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="rotateadminprovidercredential-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Rotated provider credential metadata|ProviderCredentialEnvelope|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Credential not found|Error|
|412|[Precondition Failed](https://tools.ietf.org/html/rfc7232#section-4.2)|Credential encryption is not configured|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminProviderReconciliationRuns

<a id="opIdlistAdminProviderReconciliationRuns"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/providers/reconciliation/runs',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/providers/reconciliation/runs`

<h3 id="listadminproviderreconciliationruns-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|provider_type|query|string|false|none|
|provider_id|query|string|false|none|
|page|query|integer|false|none|
|limit|query|integer|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|provider_type|payment|
|provider_type|shipping|
|provider_type|tax|

<h3 id="listadminproviderreconciliationruns-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Provider reconciliation runs|ProviderReconciliationRunPage|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminProviderReconciliationRun

<a id="opIdcreateAdminProviderReconciliationRun"></a>

> Code samples

```javascript
const inputBody = '{
  "provider_type": "payment",
  "provider_id": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/providers/reconciliation/runs',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/providers/reconciliation/runs`

> Body parameter

```json
{
  "provider_type": "payment",
  "provider_id": "string"
}
```

<h3 id="createadminproviderreconciliationrun-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|ProviderReconciliationRunRequest|true|none|

<h3 id="createadminproviderreconciliationrun-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Provider reconciliation run|ProviderReconciliationRunEnvelope|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminProviderReconciliationRun

<a id="opIdgetAdminProviderReconciliationRun"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/providers/reconciliation/runs/{id}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/providers/reconciliation/runs/{id}`

<h3 id="getadminproviderreconciliationrun-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadminproviderreconciliationrun-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Provider reconciliation run details|ProviderReconciliationRunEnvelope|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Reconciliation run not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listUsers

<a id="opIdlistUsers"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/users',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/users`

<h3 id="listusers-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|page|query|integer|false|none|
|limit|query|integer|false|none|
|q|query|string|false|none|

<h3 id="listusers-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Users page|UserPage|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateUserRole

<a id="opIdupdateUserRole"></a>

> Code samples

```javascript
const inputBody = '{
  "role": "admin"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/users/{id}/role',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/users/{id}/role`

> Body parameter

```json
{
  "role": "admin"
}
```

<h3 id="updateuserrole-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|UpdateUserRoleRequest|true|none|

<h3 id="updateuserrole-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated user|User|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCheckoutPlugins

<a id="opIdlistAdminCheckoutPlugins"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/checkout/plugins',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/checkout/plugins`

<h3 id="listadmincheckoutplugins-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Checkout providers including disabled ones|CheckoutPluginCatalog|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCheckoutPlugin

<a id="opIdupdateAdminCheckoutPlugin"></a>

> Code samples

```javascript
const inputBody = '{
  "enabled": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/checkout/plugins/{type}/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/checkout/plugins/{type}/{id}`

> Body parameter

```json
{
  "enabled": true
}
```

<h3 id="updateadmincheckoutplugin-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|type|path|string|true|none|
|id|path|string|true|none|
|body|body|UpdateCheckoutPluginRequest|true|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|type|payment|
|type|shipping|
|type|tax|

<h3 id="updateadmincheckoutplugin-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated checkout provider catalog|CheckoutPluginCatalog|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid request payload|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminInventoryReservations

<a id="opIdlistAdminInventoryReservations"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/reservations',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/inventory/reservations`

<h3 id="listadmininventoryreservations-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|status|query|array[string]|false|none|
|limit|query|integer|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|ACTIVE|
|status|CONSUMED|
|status|RELEASED|
|status|EXPIRED|

<h3 id="listadmininventoryreservations-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Inventory reservations|InventoryReservationList|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminInventoryAlerts

<a id="opIdlistAdminInventoryAlerts"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/alerts',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/inventory/alerts`

<h3 id="listadmininventoryalerts-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|status|query|array[string]|false|none|
|limit|query|integer|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|status|OPEN|
|status|ACKED|
|status|RESOLVED|

<h3 id="listadmininventoryalerts-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Inventory alerts|InventoryAlertList|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## ackAdminInventoryAlert

<a id="opIdackAdminInventoryAlert"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/alerts/{id}/ack',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/inventory/alerts/{id}/ack`

<h3 id="ackadmininventoryalert-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="ackadmininventoryalert-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Acknowledged inventory alert|InventoryAlert|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid alert id|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## resolveAdminInventoryAlert

<a id="opIdresolveAdminInventoryAlert"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/alerts/{id}/resolve',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/inventory/alerts/{id}/resolve`

<h3 id="resolveadmininventoryalert-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="resolveadmininventoryalert-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Resolved inventory alert|InventoryAlert|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid alert id|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminInventoryThresholds

<a id="opIdlistAdminInventoryThresholds"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/thresholds',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/inventory/thresholds`

<h3 id="listadmininventorythresholds-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|product_variant_id|query|integer|false|none|

<h3 id="listadmininventorythresholds-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Inventory thresholds|InventoryThresholdList|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## upsertAdminInventoryThreshold

<a id="opIdupsertAdminInventoryThreshold"></a>

> Code samples

```javascript
const inputBody = '{
  "product_variant_id": 0,
  "low_stock_quantity": 0
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/thresholds',
{
  method: 'PUT',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PUT /api/v1/admin/inventory/thresholds`

> Body parameter

```json
{
  "product_variant_id": 0,
  "low_stock_quantity": 0
}
```

<h3 id="upsertadmininventorythreshold-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|InventoryThresholdRequest|true|none|

<h3 id="upsertadmininventorythreshold-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Inventory threshold|InventoryThreshold|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid threshold request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteAdminInventoryThreshold

<a id="opIddeleteAdminInventoryThreshold"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/thresholds/{id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/inventory/thresholds/{id}`

<h3 id="deleteadmininventorythreshold-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="deleteadmininventorythreshold-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Inventory threshold deleted|MessageResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid threshold id|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminInventoryAdjustment

<a id="opIdcreateAdminInventoryAdjustment"></a>

> Code samples

```javascript
const inputBody = '{
  "product_variant_id": 0,
  "quantity_delta": 0,
  "reason_code": "CYCLE_COUNT_GAIN",
  "notes": "string",
  "approved_by_type": "string",
  "approved_by_id": 0
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/adjustments',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/inventory/adjustments`

> Body parameter

```json
{
  "product_variant_id": 0,
  "quantity_delta": 0,
  "reason_code": "CYCLE_COUNT_GAIN",
  "notes": "string",
  "approved_by_type": "string",
  "approved_by_id": 0
}
```

<h3 id="createadmininventoryadjustment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|InventoryAdjustmentRequest|true|none|

<h3 id="createadmininventoryadjustment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Inventory adjustment created|InventoryAdjustmentResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid adjustment|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## runAdminInventoryReconciliation

<a id="opIdrunAdminInventoryReconciliation"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/reconciliation',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/inventory/reconciliation`

<h3 id="runadmininventoryreconciliation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Inventory reconciliation report|InventoryReconciliationReport|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminInventoryTimeline

<a id="opIdgetAdminInventoryTimeline"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/inventory/variants/{product_variant_id}/timeline',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/inventory/variants/{product_variant_id}/timeline`

<h3 id="getadmininventorytimeline-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|product_variant_id|path|integer|true|none|
|limit|query|integer|false|none|

<h3 id="getadmininventorytimeline-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Inventory timeline|InventoryTimeline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid product variant id|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminPurchaseOrders

<a id="opIdlistAdminPurchaseOrders"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/purchase-orders',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/purchase-orders`

<h3 id="listadminpurchaseorders-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|limit|query|integer|false|none|

<h3 id="listadminpurchaseorders-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Purchase orders|PurchaseOrderList|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminPurchaseOrder

<a id="opIdcreateAdminPurchaseOrder"></a>

> Code samples

```javascript
const inputBody = '{
  "supplier_id": 0,
  "supplier": {
    "name": "string",
    "email": "string",
    "notes": "string"
  },
  "notes": "string",
  "items": [
    {
      "product_variant_id": 0,
      "quantity_ordered": 1,
      "unit_cost": 0
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/purchase-orders',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/purchase-orders`

> Body parameter

```json
{
  "supplier_id": 0,
  "supplier": {
    "name": "string",
    "email": "string",
    "notes": "string"
  },
  "notes": "string",
  "items": [
    {
      "product_variant_id": 0,
      "quantity_ordered": 1,
      "unit_cost": 0
    }
  ]
}
```

<h3 id="createadminpurchaseorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|PurchaseOrderRequest|true|none|

<h3 id="createadminpurchaseorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Purchase order created|PurchaseOrder|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid purchase order|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## issueAdminPurchaseOrder

<a id="opIdissueAdminPurchaseOrder"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/purchase-orders/{id}/issue',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/purchase-orders/{id}/issue`

<h3 id="issueadminpurchaseorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="issueadminpurchaseorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Purchase order issued|PurchaseOrder|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid purchase order transition|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## cancelAdminPurchaseOrder

<a id="opIdcancelAdminPurchaseOrder"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/purchase-orders/{id}/cancel',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/purchase-orders/{id}/cancel`

<h3 id="canceladminpurchaseorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="canceladminpurchaseorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Purchase order cancelled|PurchaseOrder|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid purchase order transition|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## receiveAdminPurchaseOrder

<a id="opIdreceiveAdminPurchaseOrder"></a>

> Code samples

```javascript
const inputBody = '{
  "notes": "string",
  "items": [
    {
      "purchase_order_item_id": 0,
      "quantity_received": 1
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/purchase-orders/{id}/receive',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/purchase-orders/{id}/receive`

> Body parameter

```json
{
  "notes": "string",
  "items": [
    {
      "purchase_order_item_id": 0,
      "quantity_received": 1
    }
  ]
}
```

<h3 id="receiveadminpurchaseorder-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|PurchaseOrderReceiveRequest|true|none|

<h3 id="receiveadminpurchaseorder-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Purchase order receipt|PurchaseOrderReceiptResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid receipt|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCmsPages

<a id="opIdlistAdminCmsPages"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/pages`

<h3 id="listadmincmspages-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|page|query|integer|false|none|
|limit|query|integer|false|none|

<h3 id="listadmincmspages-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS pages|CmsPageListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminCmsPage

<a id="opIdcreateAdminCmsPage"></a>

> Code samples

```javascript
const inputBody = '{
  "path": "string",
  "slug": "string",
  "title": "string",
  "template_key": "string",
  "visibility": "public",
  "is_homepage": true,
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/pages`

> Body parameter

```json
{
  "path": "string",
  "slug": "string",
  "title": "string",
  "template_key": "string",
  "visibility": "public",
  "is_homepage": true,
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}
```

<h3 id="createadmincmspage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsPageDraftRequest|true|none|

<h3 id="createadmincmspage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created CMS page draft|CmsPageResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## previewAdminCmsPayload

<a id="opIdpreviewAdminCmsPayload"></a>

> Code samples

```javascript
const inputBody = '{
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/preview',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/preview`

> Body parameter

```json
{
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  }
}
```

<h3 id="previewadmincmspayload-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsPreviewRequest|true|none|

<h3 id="previewadmincmspayload-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Evaluated CMS commerce block preview|CmsPreviewResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid preview request|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate path|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsPage

<a id="opIdgetAdminCmsPage"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/pages/{id}`

<h3 id="getadmincmspage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadmincmspage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS page|CmsPageResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsPage

<a id="opIdupdateAdminCmsPage"></a>

> Code samples

```javascript
const inputBody = '{
  "path": "string",
  "slug": "string",
  "title": "string",
  "template_key": "string",
  "visibility": "public",
  "is_homepage": true,
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/cms/pages/{id}`

> Body parameter

```json
{
  "path": "string",
  "slug": "string",
  "title": "string",
  "template_key": "string",
  "visibility": "public",
  "is_homepage": true,
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}
```

<h3 id="updateadmincmspage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsPageDraftRequest|true|none|

<h3 id="updateadmincmspage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated CMS page draft|CmsPageResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate path|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## publishAdminCmsPage

<a id="opIdpublishAdminCmsPage"></a>

> Code samples

```javascript
const inputBody = '{
  "notes": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/publish',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/pages/{id}/publish`

> Body parameter

```json
{
  "notes": "string"
}
```

<h3 id="publishadmincmspage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsPublishRequest|false|none|

<h3 id="publishadmincmspage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published CMS page|CmsPageResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsLocales

<a id="opIdgetAdminCmsLocales"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/locales',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/locales`

<h3 id="getadmincmslocales-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS locale registry and fallback configuration|CmsLocaleSettings|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsLocales

<a id="opIdupdateAdminCmsLocales"></a>

> Code samples

```javascript
const inputBody = '{
  "locales": [
    {
      "code": "string",
      "name": "string",
      "enabled": true,
      "is_default": true,
      "fallback_locale": "string"
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/locales',
{
  method: 'PUT',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PUT /api/v1/admin/cms/locales`

> Body parameter

```json
{
  "locales": [
    {
      "code": "string",
      "name": "string",
      "enabled": true,
      "is_default": true,
      "fallback_locale": "string"
    }
  ]
}
```

<h3 id="updateadmincmslocales-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsLocaleSettingsInput|true|none|

<h3 id="updateadmincmslocales-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated CMS locale registry|CmsLocaleSettings|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid locale configuration|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCmsPageVariants

<a id="opIdlistAdminCmsPageVariants"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/variants',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/pages/{id}/variants`

<h3 id="listadmincmspagevariants-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="listadmincmspagevariants-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Localized and market-specific page variants|Inline|

<h3 id="listadmincmspagevariants-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[CmsPageVariant]|false|none|none|
|» id|integer|true|none|none|
|» page_id|integer|true|none|none|
|» entry_id|integer|true|none|none|
|» locale|string|true|none|none|
|» market|string|true|none|none|
|» path|string|true|none|none|
|» slug|string|true|none|none|
|» title|string|true|none|none|
|» payload|CmsPagePayload|true|none|none|
|»» blocks|[oneOf]|false|none|none|

*oneOf*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsHeroBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» title|string|true|none|none|
|»»»» subtitle|string|false|none|none|
|»»»» image_media_id|string|false|none|none|
|»»»» primary_cta|CmsLink|false|none|none|
|»»»»» label|string|true|none|none|
|»»»»» url|string|true|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsRichTextBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» body|string|true|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsImageBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» media_id|string|true|none|none|
|»»»» alt|string|false|none|none|
|»»»» caption|string|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsGalleryBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» images|[CmsGalleryImage]|true|none|none|
|»»»»» media_id|string|true|none|none|
|»»»»» alt|string|false|none|none|
|»»»»» caption|string|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsVideoBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» url|string|true|none|none|
|»»»» title|string|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsFAQBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» items|[object]|true|none|none|
|»»»»» question|string|true|none|none|
|»»»»» answer|string|true|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsCTABlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» label|string|true|none|none|
|»»»» url|string|true|none|none|
|»»»» body|string|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsPromoBannerBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» title|string|true|none|none|
|»»»» body|string|false|none|none|
|»»»» link|CmsLink|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsProductRailBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» title|string|true|none|none|
|»»»» subtitle|string|false|none|none|
|»»»» source|string|true|none|none|
|»»»» product_ids|[integer]|false|none|none|
|»»»» query|string|false|none|none|
|»»»» category_slug|string|false|none|none|
|»»»» sort|string|false|none|none|
|»»»» order|string|false|none|none|
|»»»» limit|integer|true|none|none|
|»»»» show_stock|boolean|false|none|none|
|»»»» show_description|boolean|false|none|none|
|»»»» image_aspect|string|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsCategoryTilesBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» title|string|true|none|none|
|»»»» subtitle|string|false|none|none|
|»»»» category_slugs|[string]|true|none|none|
|»»»» category_media_ids|object|false|none|none|
|»»»»» **additionalProperties**|string|false|none|none|
|»»»» image_aspect|string|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsPromotionHighlightBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» title|string|true|none|none|
|»»»» body|string|false|none|none|
|»»»» badge|string|false|none|none|
|»»»» promotion_code|string|false|none|none|
|»»»» campaign_id|integer|false|none|none|
|»»»» link|CmsLink|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsInventoryMessageBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» product_id|integer|true|none|none|
|»»»» low_stock_threshold|integer|false|none|none|
|»»»» in_stock_message|string|false|none|none|
|»»»» low_stock_message|string|false|none|none|
|»»»» out_of_stock_message|string|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsTestimonialBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» quote|string|true|none|none|
|»»»» attribution|string|true|none|none|
|»»»» rating|integer|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsSocialEmbedBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» provider|string|true|none|none|
|»»»» url|string|true|none|none|
|»»»» title|string|false|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsCustomHTMLBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» html|string|true|none|none|

*xor*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|»»» *anonymous*|CmsFooterBlock|false|none|none|
|»»»» type|string|true|none|none|
|»»»» brand_name|string|true|none|none|
|»»»» tagline|string|false|none|none|
|»»»» columns|[CmsFooterColumn]|true|none|none|
|»»»»» title|string|true|none|none|
|»»»»» links|[CmsLink]|true|none|none|
|»»»» social_links|[CmsLink]|true|none|none|
|»»»» copyright|string|true|none|none|
|»»»» layout|string|true|none|none|
|»»»» theme|string|true|none|none|

*continued*

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|» status|string|true|none|none|
|» revision|integer|true|none|none|
|» submitted_by|string¦null|false|none|none|
|» approved_by|string¦null|false|none|none|
|» published_at|string(date-time)¦null|false|none|none|
|» created_at|string(date-time)|true|none|none|
|» updated_at|string(date-time)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|type|hero|
|type|rich_text|
|type|image|
|type|gallery|
|type|video|
|type|faq|
|type|cta|
|type|promo_banner|
|type|product_rail|
|source|manual|
|source|newest|
|source|search|
|source|category|
|sort|created_at|
|sort|price|
|sort|name|
|order|asc|
|order|desc|
|image_aspect|square|
|image_aspect|wide|
|type|category_tiles|
|image_aspect|square|
|image_aspect|wide|
|type|promotion_highlight|
|type|inventory_message|
|type|testimonial|
|type|social_embed|
|provider|instagram|
|provider|tiktok|
|provider|youtube|
|type|custom_html|
|type|footer|
|layout|columns|
|layout|centered|
|layout|minimal|
|theme|light|
|theme|dark|
|theme|contrast|
|status|draft|
|status|in_review|
|status|changes_requested|
|status|approved|
|status|published|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminCmsPageVariant

<a id="opIdcreateAdminCmsPageVariant"></a>

> Code samples

```javascript
const inputBody = '{
  "locale": "string",
  "market": "string",
  "path": "string",
  "slug": "string",
  "title": "string",
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/variants',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/pages/{id}/variants`

> Body parameter

```json
{
  "locale": "string",
  "market": "string",
  "path": "string",
  "slug": "string",
  "title": "string",
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}
```

<h3 id="createadmincmspagevariant-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsPageVariantInput|true|none|

<h3 id="createadmincmspagevariant-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created page variant draft|CmsPageVariant|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid page variant|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate locale and market variant or localized path|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsPageVariant

<a id="opIdupdateAdminCmsPageVariant"></a>

> Code samples

```javascript
const inputBody = '{
  "locale": "string",
  "market": "string",
  "path": "string",
  "slug": "string",
  "title": "string",
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/variants/{variant_id}',
{
  method: 'PUT',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PUT /api/v1/admin/cms/pages/{id}/variants/{variant_id}`

> Body parameter

```json
{
  "locale": "string",
  "market": "string",
  "path": "string",
  "slug": "string",
  "title": "string",
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}
```

<h3 id="updateadmincmspagevariant-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|variant_id|path|integer|true|none|
|body|body|CmsPageVariantInput|true|none|

<h3 id="updateadmincmspagevariant-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated page variant draft|CmsPageVariant|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid page variant|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteAdminCmsPageVariant

<a id="opIddeleteAdminCmsPageVariant"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/variants/{variant_id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/cms/pages/{id}/variants/{variant_id}`

<h3 id="deleteadmincmspagevariant-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|variant_id|path|integer|true|none|

<h3 id="deleteadmincmspagevariant-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted page variant|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## transitionAdminCmsPageVariant

<a id="opIdtransitionAdminCmsPageVariant"></a>

> Code samples

```javascript
const inputBody = '{
  "comment": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/variants/{variant_id}/{action}',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/pages/{id}/variants/{variant_id}/{action}`

> Body parameter

```json
{
  "comment": "string"
}
```

<h3 id="transitionadmincmspagevariant-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|variant_id|path|integer|true|none|
|action|path|string|true|none|
|body|body|CmsWorkflowActionInput|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|action|submit|
|action|approve|
|action|request_changes|
|action|publish|
|action|rollback|

<h3 id="transitionadmincmspagevariant-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Transitioned page variant workflow|CmsPageVariant|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid workflow transition|Error|
|403|[Forbidden](https://tools.ietf.org/html/rfc7231#section-6.5.3)|Insufficient CMS permission|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCmsAuditEvents

<a id="opIdlistAdminCmsAuditEvents"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/audit',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/audit`

<h3 id="listadmincmsauditevents-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|entry_id|query|integer|false|none|
|limit|query|integer|false|none|

<h3 id="listadmincmsauditevents-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS mutation and publication audit trail|Inline|

<h3 id="listadmincmsauditevents-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[CmsAuditEvent]|false|none|none|
|» id|integer|true|none|none|
|» entry_id|integer|true|none|none|
|» version_id|integer¦null|false|none|none|
|» variant_id|integer¦null|false|none|none|
|» action|string|true|none|none|
|» actor|string|true|none|none|
|» detail|string|true|none|none|
|» created_at|string(date-time)|true|none|none|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsGovernance

<a id="opIdgetAdminCmsGovernance"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/governance',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/governance`

<h3 id="getadmincmsgovernance-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS governance settings and role assignments|CmsGovernance|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsGovernance

<a id="opIdupdateAdminCmsGovernance"></a>

> Code samples

```javascript
const inputBody = '{
  "approval_required": true,
  "invalidation_webhook_url": "string",
  "roles": [
    {
      "subject": "string",
      "role": "author"
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/governance',
{
  method: 'PUT',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PUT /api/v1/admin/cms/governance`

> Body parameter

```json
{
  "approval_required": true,
  "invalidation_webhook_url": "string",
  "roles": [
    {
      "subject": "string",
      "role": "author"
    }
  ]
}
```

<h3 id="updateadmincmsgovernance-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsGovernanceInput|true|none|

<h3 id="updateadmincmsgovernance-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated CMS governance settings|CmsGovernance|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsEntryWorkflow

<a id="opIdgetAdminCmsEntryWorkflow"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/entries/{id}/workflow',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/entries/{id}/workflow`

<h3 id="getadmincmsentryworkflow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadmincmsentryworkflow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Entry workflow and comments|CmsEntryWorkflow|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## transitionAdminCmsEntryWorkflow

<a id="opIdtransitionAdminCmsEntryWorkflow"></a>

> Code samples

```javascript
const inputBody = '{
  "comment": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/entries/{id}/workflow/{action}',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/entries/{id}/workflow/{action}`

> Body parameter

```json
{
  "comment": "string"
}
```

<h3 id="transitionadmincmsentryworkflow-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|action|path|string|true|none|
|body|body|CmsWorkflowActionInput|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|action|submit|
|action|approve|
|action|request_changes|
|action|reset|

<h3 id="transitionadmincmsentryworkflow-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated entry workflow|CmsEntryWorkflow|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminCmsEntryComment

<a id="opIdcreateAdminCmsEntryComment"></a>

> Code samples

```javascript
const inputBody = '{
  "body": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/entries/{id}/comments',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/entries/{id}/comments`

> Body parameter

```json
{
  "body": "string"
}
```

<h3 id="createadmincmsentrycomment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsCommentInput|true|none|

<h3 id="createadmincmsentrycomment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Added editorial comment|CmsChangeComment|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## resolveAdminCmsComment

<a id="opIdresolveAdminCmsComment"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/comments/{id}/resolve',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/comments/{id}/resolve`

<h3 id="resolveadmincmscomment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="resolveadmincmscomment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Resolved editorial comment|CmsChangeComment|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCmsEntryVariants

<a id="opIdlistAdminCmsEntryVariants"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/entries/{id}/variants',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/entries/{id}/variants`

<h3 id="listadmincmsentryvariants-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="listadmincmsentryvariants-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Localized entry variants|Inline|

<h3 id="listadmincmsentryvariants-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[CmsEntryVariant]|false|none|none|
|» id|integer|true|none|none|
|» entry_id|integer|true|none|none|
|» locale|string|true|none|none|
|» market|string|true|none|none|
|» payload|object|true|none|none|
|» status|string|true|none|none|
|» revision|integer|true|none|none|
|» submitted_by|string¦null|false|none|none|
|» approved_by|string¦null|false|none|none|
|» published_at|string(date-time)¦null|false|none|none|
|» created_at|string(date-time)|true|none|none|
|» updated_at|string(date-time)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|status|draft|
|status|in_review|
|status|changes_requested|
|status|approved|
|status|published|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminCmsEntryVariant

<a id="opIdcreateAdminCmsEntryVariant"></a>

> Code samples

```javascript
const inputBody = '{
  "locale": "string",
  "market": "string",
  "payload": {},
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/entries/{id}/variants',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/entries/{id}/variants`

> Body parameter

```json
{
  "locale": "string",
  "market": "string",
  "payload": {},
  "change_summary": "string"
}
```

<h3 id="createadmincmsentryvariant-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsEntryVariantInput|true|none|

<h3 id="createadmincmsentryvariant-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created localized entry variant|CmsEntryVariant|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsEntryVariant

<a id="opIdupdateAdminCmsEntryVariant"></a>

> Code samples

```javascript
const inputBody = '{
  "locale": "string",
  "market": "string",
  "payload": {},
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/entries/{id}/variants/{variant_id}',
{
  method: 'PUT',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PUT /api/v1/admin/cms/entries/{id}/variants/{variant_id}`

> Body parameter

```json
{
  "locale": "string",
  "market": "string",
  "payload": {},
  "change_summary": "string"
}
```

<h3 id="updateadmincmsentryvariant-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|variant_id|path|integer|true|none|
|body|body|CmsEntryVariantInput|true|none|

<h3 id="updateadmincmsentryvariant-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated localized entry variant|CmsEntryVariant|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteAdminCmsEntryVariant

<a id="opIddeleteAdminCmsEntryVariant"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/entries/{id}/variants/{variant_id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/cms/entries/{id}/variants/{variant_id}`

<h3 id="deleteadmincmsentryvariant-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|variant_id|path|integer|true|none|

<h3 id="deleteadmincmsentryvariant-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Deleted localized entry variant|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## transitionAdminCmsEntryVariant

<a id="opIdtransitionAdminCmsEntryVariant"></a>

> Code samples

```javascript
const inputBody = '{
  "comment": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/entries/{id}/variants/{variant_id}/{action}',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/entries/{id}/variants/{variant_id}/{action}`

> Body parameter

```json
{
  "comment": "string"
}
```

<h3 id="transitionadmincmsentryvariant-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|variant_id|path|integer|true|none|
|action|path|string|true|none|
|body|body|CmsWorkflowActionInput|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|action|submit|
|action|approve|
|action|request_changes|
|action|publish|
|action|reset|

<h3 id="transitionadmincmsentryvariant-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Transitioned localized entry variant|CmsEntryVariant|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsOperations

<a id="opIdgetAdminCmsOperations"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/operations',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/operations`

<h3 id="getadmincmsoperations-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS publish queue and invalidation operations|CmsOperations|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## retryAdminCmsInvalidation

<a id="opIdretryAdminCmsInvalidation"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/operations/invalidation/{id}/retry',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/operations/invalidation/{id}/retry`

<h3 id="retryadmincmsinvalidation-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="retryadmincmsinvalidation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Invalidation queued for retry|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## previewAdminCmsRestore

<a id="opIdpreviewAdminCmsRestore"></a>

> Code samples

```javascript
const inputBody = '{
  "schema_version": 0,
  "exported_at": "2019-08-24T14:15:22Z",
  "locales": [
    {
      "code": "string",
      "name": "string",
      "enabled": true,
      "is_default": true,
      "fallback_locale": "string"
    }
  ],
  "pages": [
    {
      "page": {
        "id": 1,
        "entry_id": 1,
        "path": "string",
        "slug": "string",
        "title": "string",
        "template_key": "string",
        "visibility": "public",
        "seo_metadata_id": 0,
        "is_homepage": true,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true,
      "delivery": {
        "content_version_id": 1,
        "experiment_id": 1,
        "experiment_variant_id": 1,
        "correlation_id": "string"
      },
      "seo": {
        "title": "string",
        "description": "string",
        "canonical_url": "string",
        "robots": "index_follow",
        "og_title": "string",
        "og_description": "string",
        "og_image_media_id": "string",
        "twitter_card": "summary",
        "twitter_title": "string",
        "twitter_description": "string",
        "twitter_image_media_id": "string",
        "json_ld": [
          {}
        ]
      },
      "localization": {
        "requested_locale": "string",
        "resolved_locale": "string",
        "market": "string",
        "used_fallback": true,
        "alternates": [
          {
            "locale": "string",
            "market": "string",
            "path": "string"
          }
        ]
      }
    }
  ],
  "navigation": [
    {
      "menu": {
        "id": 1,
        "entry_id": 1,
        "key": "string",
        "title": "string",
        "location": "string",
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "items": [
        {
          "id": 0,
          "menu_id": 0,
          "parent_id": 0,
          "label": "string",
          "item_type": "internal",
          "target_ref": "string",
          "url": "string",
          "sort_order": 0,
          "is_enabled": true
        }
      ],
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true
    }
  ],
  "global_regions": [
    {
      "region": {
        "id": 1,
        "entry_id": 1,
        "key": "string",
        "title": "string",
        "region": "string",
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true
    }
  ],
  "variants": [
    {
      "id": 1,
      "page_id": 1,
      "entry_id": 1,
      "locale": "string",
      "market": "string",
      "path": "string",
      "slug": "string",
      "title": "string",
      "payload": {
        "blocks": [
          {
            "type": "hero",
            "title": "string",
            "subtitle": "string",
            "image_media_id": "string",
            "primary_cta": {
              "label": "string",
              "url": "string"
            }
          }
        ]
      },
      "status": "draft",
      "revision": 1,
      "submitted_by": "string",
      "approved_by": "string",
      "published_at": "2019-08-24T14:15:22Z",
      "created_at": "2019-08-24T14:15:22Z",
      "updated_at": "2019-08-24T14:15:22Z"
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/restore/preview',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/restore/preview`

> Body parameter

```json
{
  "schema_version": 0,
  "exported_at": "2019-08-24T14:15:22Z",
  "locales": [
    {
      "code": "string",
      "name": "string",
      "enabled": true,
      "is_default": true,
      "fallback_locale": "string"
    }
  ],
  "pages": [
    {
      "page": {
        "id": 1,
        "entry_id": 1,
        "path": "string",
        "slug": "string",
        "title": "string",
        "template_key": "string",
        "visibility": "public",
        "seo_metadata_id": 0,
        "is_homepage": true,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true,
      "delivery": {
        "content_version_id": 1,
        "experiment_id": 1,
        "experiment_variant_id": 1,
        "correlation_id": "string"
      },
      "seo": {
        "title": "string",
        "description": "string",
        "canonical_url": "string",
        "robots": "index_follow",
        "og_title": "string",
        "og_description": "string",
        "og_image_media_id": "string",
        "twitter_card": "summary",
        "twitter_title": "string",
        "twitter_description": "string",
        "twitter_image_media_id": "string",
        "json_ld": [
          {}
        ]
      },
      "localization": {
        "requested_locale": "string",
        "resolved_locale": "string",
        "market": "string",
        "used_fallback": true,
        "alternates": [
          {
            "locale": "string",
            "market": "string",
            "path": "string"
          }
        ]
      }
    }
  ],
  "navigation": [
    {
      "menu": {
        "id": 1,
        "entry_id": 1,
        "key": "string",
        "title": "string",
        "location": "string",
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "items": [
        {
          "id": 0,
          "menu_id": 0,
          "parent_id": 0,
          "label": "string",
          "item_type": "internal",
          "target_ref": "string",
          "url": "string",
          "sort_order": 0,
          "is_enabled": true
        }
      ],
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true
    }
  ],
  "global_regions": [
    {
      "region": {
        "id": 1,
        "entry_id": 1,
        "key": "string",
        "title": "string",
        "region": "string",
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true
    }
  ],
  "variants": [
    {
      "id": 1,
      "page_id": 1,
      "entry_id": 1,
      "locale": "string",
      "market": "string",
      "path": "string",
      "slug": "string",
      "title": "string",
      "payload": {
        "blocks": [
          {
            "type": "hero",
            "title": "string",
            "subtitle": "string",
            "image_media_id": "string",
            "primary_cta": {
              "label": "string",
              "url": "string"
            }
          }
        ]
      },
      "status": "draft",
      "revision": 1,
      "submitted_by": "string",
      "approved_by": "string",
      "published_at": "2019-08-24T14:15:22Z",
      "created_at": "2019-08-24T14:15:22Z",
      "updated_at": "2019-08-24T14:15:22Z"
    }
  ]
}
```

<h3 id="previewadmincmsrestore-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsContentExport|true|none|

<h3 id="previewadmincmsrestore-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS restore dry-run report|CmsRestorePreview|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## exportAdminCmsContent

<a id="opIdexportAdminCmsContent"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/export',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/export`

<h3 id="exportadmincmscontent-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Portable CMS backup export|CmsContentExport|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## restoreAdminCmsContent

<a id="opIdrestoreAdminCmsContent"></a>

> Code samples

```javascript
const inputBody = '{
  "schema_version": 0,
  "exported_at": "2019-08-24T14:15:22Z",
  "locales": [
    {
      "code": "string",
      "name": "string",
      "enabled": true,
      "is_default": true,
      "fallback_locale": "string"
    }
  ],
  "pages": [
    {
      "page": {
        "id": 1,
        "entry_id": 1,
        "path": "string",
        "slug": "string",
        "title": "string",
        "template_key": "string",
        "visibility": "public",
        "seo_metadata_id": 0,
        "is_homepage": true,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true,
      "delivery": {
        "content_version_id": 1,
        "experiment_id": 1,
        "experiment_variant_id": 1,
        "correlation_id": "string"
      },
      "seo": {
        "title": "string",
        "description": "string",
        "canonical_url": "string",
        "robots": "index_follow",
        "og_title": "string",
        "og_description": "string",
        "og_image_media_id": "string",
        "twitter_card": "summary",
        "twitter_title": "string",
        "twitter_description": "string",
        "twitter_image_media_id": "string",
        "json_ld": [
          {}
        ]
      },
      "localization": {
        "requested_locale": "string",
        "resolved_locale": "string",
        "market": "string",
        "used_fallback": true,
        "alternates": [
          {
            "locale": "string",
            "market": "string",
            "path": "string"
          }
        ]
      }
    }
  ],
  "navigation": [
    {
      "menu": {
        "id": 1,
        "entry_id": 1,
        "key": "string",
        "title": "string",
        "location": "string",
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "items": [
        {
          "id": 0,
          "menu_id": 0,
          "parent_id": 0,
          "label": "string",
          "item_type": "internal",
          "target_ref": "string",
          "url": "string",
          "sort_order": 0,
          "is_enabled": true
        }
      ],
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true
    }
  ],
  "global_regions": [
    {
      "region": {
        "id": 1,
        "entry_id": 1,
        "key": "string",
        "title": "string",
        "region": "string",
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true
    }
  ],
  "variants": [
    {
      "id": 1,
      "page_id": 1,
      "entry_id": 1,
      "locale": "string",
      "market": "string",
      "path": "string",
      "slug": "string",
      "title": "string",
      "payload": {
        "blocks": [
          {
            "type": "hero",
            "title": "string",
            "subtitle": "string",
            "image_media_id": "string",
            "primary_cta": {
              "label": "string",
              "url": "string"
            }
          }
        ]
      },
      "status": "draft",
      "revision": 1,
      "submitted_by": "string",
      "approved_by": "string",
      "published_at": "2019-08-24T14:15:22Z",
      "created_at": "2019-08-24T14:15:22Z",
      "updated_at": "2019-08-24T14:15:22Z"
    }
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/export',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/export`

> Body parameter

```json
{
  "schema_version": 0,
  "exported_at": "2019-08-24T14:15:22Z",
  "locales": [
    {
      "code": "string",
      "name": "string",
      "enabled": true,
      "is_default": true,
      "fallback_locale": "string"
    }
  ],
  "pages": [
    {
      "page": {
        "id": 1,
        "entry_id": 1,
        "path": "string",
        "slug": "string",
        "title": "string",
        "template_key": "string",
        "visibility": "public",
        "seo_metadata_id": 0,
        "is_homepage": true,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true,
      "delivery": {
        "content_version_id": 1,
        "experiment_id": 1,
        "experiment_variant_id": 1,
        "correlation_id": "string"
      },
      "seo": {
        "title": "string",
        "description": "string",
        "canonical_url": "string",
        "robots": "index_follow",
        "og_title": "string",
        "og_description": "string",
        "og_image_media_id": "string",
        "twitter_card": "summary",
        "twitter_title": "string",
        "twitter_description": "string",
        "twitter_image_media_id": "string",
        "json_ld": [
          {}
        ]
      },
      "localization": {
        "requested_locale": "string",
        "resolved_locale": "string",
        "market": "string",
        "used_fallback": true,
        "alternates": [
          {
            "locale": "string",
            "market": "string",
            "path": "string"
          }
        ]
      }
    }
  ],
  "navigation": [
    {
      "menu": {
        "id": 1,
        "entry_id": 1,
        "key": "string",
        "title": "string",
        "location": "string",
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "items": [
        {
          "id": 0,
          "menu_id": 0,
          "parent_id": 0,
          "label": "string",
          "item_type": "internal",
          "target_ref": "string",
          "url": "string",
          "sort_order": 0,
          "is_enabled": true
        }
      ],
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true
    }
  ],
  "global_regions": [
    {
      "region": {
        "id": 1,
        "entry_id": 1,
        "key": "string",
        "title": "string",
        "region": "string",
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "entry": {
        "id": 1,
        "entry_type": "page",
        "key": "string",
        "status": "DRAFT",
        "current_version_id": 0,
        "published_version_id": 0,
        "created_at": "2019-08-24T14:15:22Z",
        "updated_at": "2019-08-24T14:15:22Z"
      },
      "current_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "published_version": {
        "id": 1,
        "entry_id": 1,
        "version_number": 1,
        "schema_version": 1,
        "payload": {
          "blocks": [
            {
              "type": "hero",
              "title": "string",
              "subtitle": "string",
              "image_media_id": "string",
              "primary_cta": {
                "label": "string",
                "url": "string"
              }
            }
          ]
        },
        "created_by": 0,
        "change_summary": "string",
        "created_at": "2019-08-24T14:15:22Z"
      },
      "latest_publication": {
        "id": 1,
        "entry_id": 1,
        "version_id": 1,
        "published_by": 0,
        "published_at": "2019-08-24T14:15:22Z",
        "rollback_from_publication_id": 0,
        "notes": "string"
      },
      "has_unpublished_draft": true
    }
  ],
  "variants": [
    {
      "id": 1,
      "page_id": 1,
      "entry_id": 1,
      "locale": "string",
      "market": "string",
      "path": "string",
      "slug": "string",
      "title": "string",
      "payload": {
        "blocks": [
          {
            "type": "hero",
            "title": "string",
            "subtitle": "string",
            "image_media_id": "string",
            "primary_cta": {
              "label": "string",
              "url": "string"
            }
          }
        ]
      },
      "status": "draft",
      "revision": 1,
      "submitted_by": "string",
      "approved_by": "string",
      "published_at": "2019-08-24T14:15:22Z",
      "created_at": "2019-08-24T14:15:22Z",
      "updated_at": "2019-08-24T14:15:22Z"
    }
  ]
}
```

<h3 id="restoreadmincmscontent-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsContentExport|true|none|

<h3 id="restoreadmincmscontent-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS content restored from export|MessageResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid or unsupported CMS export|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## rollbackAdminCmsPage

<a id="opIdrollbackAdminCmsPage"></a>

> Code samples

```javascript
const inputBody = '{
  "version_id": 1,
  "notes": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/rollback',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/pages/{id}/rollback`

> Body parameter

```json
{
  "version_id": 1,
  "notes": "string"
}
```

<h3 id="rollbackadmincmspage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsRollbackRequest|true|none|

<h3 id="rollbackadmincmspage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Rolled back CMS page|CmsPageResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsPageDelivery

<a id="opIdgetAdminCmsPageDelivery"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/delivery',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/pages/{id}/delivery`

<h3 id="getadmincmspagedelivery-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadmincmspagedelivery-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS page scheduling, targeting, and experiment settings|CmsPageDeliveryResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsPageDelivery

<a id="opIdupdateAdminCmsPageDelivery"></a>

> Code samples

```javascript
const inputBody = '{
  "schedule": {
    "publish_at": "2019-08-24T14:15:22Z",
    "unpublish_at": "2019-08-24T14:15:22Z",
    "timezone": "string"
  },
  "targeting_rules": [
    {
      "priority": 0,
      "is_enabled": true,
      "markets": [
        "string"
      ],
      "device_classes": [
        "desktop"
      ],
      "auth_states": [
        "guest"
      ],
      "referrers": [
        "string"
      ],
      "utm_sources": [
        "string"
      ],
      "segment_keys": [
        "string"
      ]
    }
  ],
  "experiment": {
    "name": "string",
    "status": "draft",
    "sticky_key": "visitor",
    "starts_at": "2019-08-24T14:15:22Z",
    "ends_at": "2019-08-24T14:15:22Z",
    "variants": [
      {
        "name": "string",
        "version_id": 1,
        "allocation": 1
      },
      {
        "name": "string",
        "version_id": 1,
        "allocation": 1
      }
    ]
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/delivery',
{
  method: 'PUT',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PUT /api/v1/admin/cms/pages/{id}/delivery`

> Body parameter

```json
{
  "schedule": {
    "publish_at": "2019-08-24T14:15:22Z",
    "unpublish_at": "2019-08-24T14:15:22Z",
    "timezone": "string"
  },
  "targeting_rules": [
    {
      "priority": 0,
      "is_enabled": true,
      "markets": [
        "string"
      ],
      "device_classes": [
        "desktop"
      ],
      "auth_states": [
        "guest"
      ],
      "referrers": [
        "string"
      ],
      "utm_sources": [
        "string"
      ],
      "segment_keys": [
        "string"
      ]
    }
  ],
  "experiment": {
    "name": "string",
    "status": "draft",
    "sticky_key": "visitor",
    "starts_at": "2019-08-24T14:15:22Z",
    "ends_at": "2019-08-24T14:15:22Z",
    "variants": [
      {
        "name": "string",
        "version_id": 1,
        "allocation": 1
      },
      {
        "name": "string",
        "version_id": 1,
        "allocation": 1
      }
    ]
  }
}
```

<h3 id="updateadmincmspagedelivery-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsPageDeliveryRequest|true|none|

<h3 id="updateadmincmspagedelivery-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated CMS page delivery settings|CmsPageDeliveryResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid delivery settings|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsPageSeo

<a id="opIdgetAdminCmsPageSeo"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/seo',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/pages/{id}/seo`

<h3 id="getadmincmspageseo-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadmincmspageseo-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS page SEO metadata and validation|CmsSEOResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsPageSeo

<a id="opIdupdateAdminCmsPageSeo"></a>

> Code samples

```javascript
const inputBody = '{
  "title": "string",
  "description": "string",
  "canonical_url": "string",
  "robots": "index_follow",
  "og_title": "string",
  "og_description": "string",
  "og_image_media_id": "string",
  "twitter_card": "summary",
  "twitter_title": "string",
  "twitter_description": "string",
  "twitter_image_media_id": "string",
  "json_ld": [
    {}
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/pages/{id}/seo',
{
  method: 'PUT',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PUT /api/v1/admin/cms/pages/{id}/seo`

> Body parameter

```json
{
  "title": "string",
  "description": "string",
  "canonical_url": "string",
  "robots": "index_follow",
  "og_title": "string",
  "og_description": "string",
  "og_image_media_id": "string",
  "twitter_card": "summary",
  "twitter_title": "string",
  "twitter_description": "string",
  "twitter_image_media_id": "string",
  "json_ld": [
    {}
  ]
}
```

<h3 id="updateadmincmspageseo-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsSEOInput|true|none|

<h3 id="updateadmincmspageseo-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated CMS page SEO metadata|CmsSEOResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid SEO metadata|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCmsRedirects

<a id="opIdlistAdminCmsRedirects"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/redirects',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/redirects`

<h3 id="listadmincmsredirects-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS redirect rules|Inline|

<h3 id="listadmincmsredirects-responseschema">Response Schema</h3>

Status Code **200**

|Name|Type|Required|Restrictions|Description|
|---|---|---|---|---|
|*anonymous*|[CmsRedirectRule]|false|none|none|
|» id|integer|true|none|none|
|» source_pattern|string|true|none|none|
|» match_type|string|true|none|none|
|» target_url|string|true|none|none|
|» redirect_type|integer|true|none|none|
|» priority|integer|true|none|none|
|» is_enabled|boolean|true|none|none|
|» created_at|string(date-time)|true|none|none|
|» updated_at|string(date-time)|true|none|none|

#### Enumerated Values

|Property|Value|
|---|---|
|match_type|exact|
|match_type|prefix|
|redirect_type|301|
|redirect_type|302|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminCmsRedirect

<a id="opIdcreateAdminCmsRedirect"></a>

> Code samples

```javascript
const inputBody = '{
  "source_pattern": "string",
  "match_type": "exact",
  "target_url": "string",
  "redirect_type": 301,
  "priority": 0,
  "is_enabled": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/redirects',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/redirects`

> Body parameter

```json
{
  "source_pattern": "string",
  "match_type": "exact",
  "target_url": "string",
  "redirect_type": 301,
  "priority": 0,
  "is_enabled": true
}
```

<h3 id="createadmincmsredirect-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsRedirectInput|true|none|

<h3 id="createadmincmsredirect-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created redirect rule|CmsRedirectRule|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid redirect|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsRedirect

<a id="opIdupdateAdminCmsRedirect"></a>

> Code samples

```javascript
const inputBody = '{
  "source_pattern": "string",
  "match_type": "exact",
  "target_url": "string",
  "redirect_type": 301,
  "priority": 0,
  "is_enabled": true
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/redirects/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/cms/redirects/{id}`

> Body parameter

```json
{
  "source_pattern": "string",
  "match_type": "exact",
  "target_url": "string",
  "redirect_type": 301,
  "priority": 0,
  "is_enabled": true
}
```

<h3 id="updateadmincmsredirect-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsRedirectInput|true|none|

<h3 id="updateadmincmsredirect-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated redirect rule|CmsRedirectRule|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid redirect|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteAdminCmsRedirect

<a id="opIddeleteAdminCmsRedirect"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/redirects/{id}',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/admin/cms/redirects/{id}`

<h3 id="deleteadmincmsredirect-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="deleteadmincmsredirect-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Redirect deleted|MessageResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCmsNavigation

<a id="opIdlistAdminCmsNavigation"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/navigation',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/navigation`

<h3 id="listadmincmsnavigation-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|page|query|integer|false|none|
|limit|query|integer|false|none|

<h3 id="listadmincmsnavigation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS navigation menus|CmsNavigationListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminCmsNavigation

<a id="opIdcreateAdminCmsNavigation"></a>

> Code samples

```javascript
const inputBody = '{
  "key": "string",
  "title": "string",
  "location": "string",
  "items": [
    {
      "id": 0,
      "parent_id": 0,
      "label": "string",
      "item_type": "internal",
      "target_ref": "string",
      "url": "string",
      "sort_order": 0,
      "is_enabled": true
    }
  ],
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/navigation',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/navigation`

> Body parameter

```json
{
  "key": "string",
  "title": "string",
  "location": "string",
  "items": [
    {
      "id": 0,
      "parent_id": 0,
      "label": "string",
      "item_type": "internal",
      "target_ref": "string",
      "url": "string",
      "sort_order": 0,
      "is_enabled": true
    }
  ],
  "change_summary": "string"
}
```

<h3 id="createadmincmsnavigation-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsNavigationDraftRequest|true|none|

<h3 id="createadmincmsnavigation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created navigation draft|CmsNavigationResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate key|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsNavigation

<a id="opIdgetAdminCmsNavigation"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/navigation/{id}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/navigation/{id}`

<h3 id="getadmincmsnavigation-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadmincmsnavigation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS navigation menu|CmsNavigationResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsNavigation

<a id="opIdupdateAdminCmsNavigation"></a>

> Code samples

```javascript
const inputBody = '{
  "key": "string",
  "title": "string",
  "location": "string",
  "items": [
    {
      "id": 0,
      "parent_id": 0,
      "label": "string",
      "item_type": "internal",
      "target_ref": "string",
      "url": "string",
      "sort_order": 0,
      "is_enabled": true
    }
  ],
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/navigation/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/cms/navigation/{id}`

> Body parameter

```json
{
  "key": "string",
  "title": "string",
  "location": "string",
  "items": [
    {
      "id": 0,
      "parent_id": 0,
      "label": "string",
      "item_type": "internal",
      "target_ref": "string",
      "url": "string",
      "sort_order": 0,
      "is_enabled": true
    }
  ],
  "change_summary": "string"
}
```

<h3 id="updateadmincmsnavigation-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsNavigationDraftRequest|true|none|

<h3 id="updateadmincmsnavigation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated navigation draft|CmsNavigationResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate key|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## publishAdminCmsNavigation

<a id="opIdpublishAdminCmsNavigation"></a>

> Code samples

```javascript
const inputBody = '{
  "notes": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/navigation/{id}/publish',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/navigation/{id}/publish`

> Body parameter

```json
{
  "notes": "string"
}
```

<h3 id="publishadmincmsnavigation-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsPublishRequest|false|none|

<h3 id="publishadmincmsnavigation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published navigation|CmsNavigationResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## listAdminCmsGlobalRegions

<a id="opIdlistAdminCmsGlobalRegions"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/global',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/global`

<h3 id="listadmincmsglobalregions-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|page|query|integer|false|none|
|limit|query|integer|false|none|

<h3 id="listadmincmsglobalregions-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS global regions|CmsGlobalRegionListResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createAdminCmsGlobalRegion

<a id="opIdcreateAdminCmsGlobalRegion"></a>

> Code samples

```javascript
const inputBody = '{
  "key": "string",
  "title": "string",
  "region": "string",
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/global',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/global`

> Body parameter

```json
{
  "key": "string",
  "title": "string",
  "region": "string",
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}
```

<h3 id="createadmincmsglobalregion-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsGlobalRegionDraftRequest|true|none|

<h3 id="createadmincmsglobalregion-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created global region draft|CmsGlobalRegionResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate key|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminCmsGlobalRegion

<a id="opIdgetAdminCmsGlobalRegion"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/global/{id}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/cms/global/{id}`

<h3 id="getadmincmsglobalregion-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|

<h3 id="getadmincmsglobalregion-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|CMS global region|CmsGlobalRegionResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateAdminCmsGlobalRegion

<a id="opIdupdateAdminCmsGlobalRegion"></a>

> Code samples

```javascript
const inputBody = '{
  "key": "string",
  "title": "string",
  "region": "string",
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/global/{id}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/admin/cms/global/{id}`

> Body parameter

```json
{
  "key": "string",
  "title": "string",
  "region": "string",
  "payload": {
    "blocks": [
      {
        "type": "hero",
        "title": "string",
        "subtitle": "string",
        "image_media_id": "string",
        "primary_cta": {
          "label": "string",
          "url": "string"
        }
      }
    ]
  },
  "change_summary": "string"
}
```

<h3 id="updateadmincmsglobalregion-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsGlobalRegionDraftRequest|true|none|

<h3 id="updateadmincmsglobalregion-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated global region draft|CmsGlobalRegionResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Duplicate key|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## publishAdminCmsGlobalRegion

<a id="opIdpublishAdminCmsGlobalRegion"></a>

> Code samples

```javascript
const inputBody = '{
  "notes": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/cms/global/{id}/publish',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/cms/global/{id}/publish`

> Body parameter

```json
{
  "notes": "string"
}
```

<h3 id="publishadmincmsglobalregion-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|CmsPublishRequest|false|none|

<h3 id="publishadmincmsglobalregion-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published global region|CmsGlobalRegionResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminWebsiteSettings

<a id="opIdgetAdminWebsiteSettings"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/website',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/website`

<h3 id="getadminwebsitesettings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Website settings|WebsiteSettingsResponse|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateWebsiteSettings

<a id="opIdupdateWebsiteSettings"></a>

> Code samples

```javascript
const inputBody = '{
  "settings": {
    "site_title": "string",
    "allow_guest_checkout": true,
    "coupon_codes_enabled": true,
    "oidc_provider": "string",
    "oidc_client_id": "string",
    "oidc_client_secret": "string",
    "oidc_client_secret_configured": true,
    "clear_oidc_client_secret": true,
    "oidc_redirect_uri": "string"
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/website',
{
  method: 'PUT',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PUT /api/v1/admin/website`

> Body parameter

```json
{
  "settings": {
    "site_title": "string",
    "allow_guest_checkout": true,
    "coupon_codes_enabled": true,
    "oidc_provider": "string",
    "oidc_client_id": "string",
    "oidc_client_secret": "string",
    "oidc_client_secret_configured": true,
    "clear_oidc_client_secret": true,
    "oidc_redirect_uri": "string"
  }
}
```

<h3 id="updatewebsitesettings-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|WebsiteSettingsRequest|true|none|

<h3 id="updatewebsitesettings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated website settings|WebsiteSettingsResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## getAdminPreview

<a id="opIdgetAdminPreview"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/preview',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/admin/preview`

<h3 id="getadminpreview-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Current preview session state|DraftPreviewSessionResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## startAdminPreview

<a id="opIdstartAdminPreview"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/preview/start',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/preview/start`

<h3 id="startadminpreview-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Preview session started|DraftPreviewSessionResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## stopAdminPreview

<a id="opIdstopAdminPreview"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/preview/stop',
{
  method: 'POST',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/admin/preview/stop`

<h3 id="stopadminpreview-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Preview session stopped|DraftPreviewSessionResponse|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

<h1 id="ecommerce-api-media">media</h1>

## setProfilePhoto

<a id="opIdsetProfilePhoto"></a>

> Code samples

```javascript
const inputBody = '{
  "media_id": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/profile-photo',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/me/profile-photo`

> Body parameter

```json
{
  "media_id": "string"
}
```

<h3 id="setprofilephoto-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|object|true|none|
|» media_id|body|string|true|none|

<h3 id="setprofilephoto-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated profile|User|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|409|[Conflict](https://tools.ietf.org/html/rfc7231#section-6.5.8)|Conflict|Error|
|413|[Payload Too Large](https://tools.ietf.org/html/rfc7231#section-6.5.11)|Too large|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## deleteProfilePhoto

<a id="opIddeleteProfilePhoto"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/me/profile-photo',
{
  method: 'DELETE',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`DELETE /api/v1/me/profile-photo`

<h3 id="deleteprofilephoto-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated profile|User|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## createMediaUpload

<a id="opIdcreateMediaUpload"></a>

> Code samples

```javascript
const inputBody = 'string';
const headers = {
  'Content-Type':'application/offset+octet-stream',
  'Tus-Resumable':'string',
  'Upload-Length':'0',
  'Upload-Metadata':'string'
};

fetch('http://localhost:3000/api/v1/media/uploads',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/media/uploads`

> Body parameter

<h3 id="createmediaupload-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|Tus-Resumable|header|string|false|none|
|Upload-Length|header|integer|false|none|
|Upload-Metadata|header|string|false|none|
|body|body|string(binary)|false|none|

<h3 id="createmediaupload-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Upload created|None|

### Response Headers

|Status|Header|Type|Format|Description|
|---|---|---|---|---|
|201|Location|string||none|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## patchMediaUpload

<a id="opIdpatchMediaUpload"></a>

> Code samples

```javascript
const inputBody = 'string';
const headers = {
  'Content-Type':'application/offset+octet-stream'
};

fetch('http://localhost:3000/api/v1/media/uploads/{path}',
{
  method: 'PATCH',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`PATCH /api/v1/media/uploads/{path}`

> Body parameter

<h3 id="patchmediaupload-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|path|path|string|true|none|
|body|body|string(binary)|true|none|

<h3 id="patchmediaupload-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|204|[No Content](https://tools.ietf.org/html/rfc7231#section-6.3.5)|Upload chunk accepted|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## headMediaUpload

<a id="opIdheadMediaUpload"></a>

> Code samples

```javascript

fetch('http://localhost:3000/api/v1/media/uploads/{path}',
{
  method: 'HEAD'

})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`HEAD /api/v1/media/uploads/{path}`

<h3 id="headmediaupload-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|path|path|string|true|none|

<h3 id="headmediaupload-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Upload status|None|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

<h1 id="ecommerce-api-cms">cms</h1>

## resolveContentHomepage

<a id="opIdresolveContentHomepage"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/content',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/content`

<h3 id="resolvecontenthomepage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|market|query|string|false|none|
|locale|query|string|false|none|
|device|query|string|false|none|
|segment|query|string|false|none|
|utm_source|query|string|false|none|
|assignment_key|query|string|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|device|desktop|
|device|mobile|
|device|tablet|

<h3 id="resolvecontenthomepage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published CMS homepage|CmsPageResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="success">
This operation does not require authentication
</aside>

## resolveContentPage

<a id="opIdresolveContentPage"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/content/{path}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/content/{path}`

<h3 id="resolvecontentpage-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|path|path|string|true|none|
|market|query|string|false|none|
|locale|query|string|false|none|
|device|query|string|false|none|
|segment|query|string|false|none|
|utm_source|query|string|false|none|
|assignment_key|query|string|false|none|

#### Enumerated Values

|Parameter|Value|
|---|---|
|device|desktop|
|device|mobile|
|device|tablet|

<h3 id="resolvecontentpage-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published CMS page|CmsPageResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="success">
This operation does not require authentication
</aside>

## recordContentEvent

<a id="opIdrecordContentEvent"></a>

> Code samples

```javascript
const inputBody = '{
  "content_version_id": 1,
  "experiment_id": 1,
  "experiment_variant_id": 1,
  "correlation_id": "string",
  "event_type": "impression"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/content/events',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/content/events`

> Body parameter

```json
{
  "content_version_id": 1,
  "experiment_id": 1,
  "experiment_variant_id": 1,
  "correlation_id": "string",
  "event_type": "impression"
}
```

<h3 id="recordcontentevent-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|CmsContentEventRequest|true|none|

<h3 id="recordcontentevent-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|202|[Accepted](https://tools.ietf.org/html/rfc7231#section-6.3.3)|Content event accepted|MessageResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Invalid content event|Error|

<aside class="success">
This operation does not require authentication
</aside>

## resolveContentRedirect

<a id="opIdresolveContentRedirect"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/content/redirect?path=string',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/content/redirect`

<h3 id="resolvecontentredirect-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|path|query|string|true|none|

<h3 id="resolvecontentredirect-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Matching redirect|CmsRedirectResolution|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|No redirect|Error|

<aside class="success">
This operation does not require authentication
</aside>

## getContentSitemap

<a id="opIdgetContentSitemap"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/xml'
};

fetch('http://localhost:3000/api/v1/content/sitemap.xml',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/content/sitemap.xml`

<h3 id="getcontentsitemap-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Storefront sitemap|string|

<aside class="success">
This operation does not require authentication
</aside>

## getContentNavigation

<a id="opIdgetContentNavigation"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/content/navigation/{location}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/content/navigation/{location}`

<h3 id="getcontentnavigation-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|location|path|string|true|none|

<h3 id="getcontentnavigation-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published CMS navigation menu|CmsNavigationResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="success">
This operation does not require authentication
</aside>

## getContentGlobalRegion

<a id="opIdgetContentGlobalRegion"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/content/global/{region}',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/content/global/{region}`

<h3 id="getcontentglobalregion-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|region|path|string|true|none|

<h3 id="getcontentglobalregion-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published CMS global region|CmsGlobalRegionResponse|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Not found|Error|

<aside class="success">
This operation does not require authentication
</aside>

<h1 id="ecommerce-api-catalog">catalog</h1>

## List active storefront categories

<a id="opIdlistCategories"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/categories',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`GET /api/v1/categories`

<h3 id="list-active-storefront-categories-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Active categories|CategoryListResponse|

<aside class="success">
This operation does not require authentication
</aside>

<h1 id="ecommerce-api-webhooks">webhooks</h1>

## receiveWebhookEvent

<a id="opIdreceiveWebhookEvent"></a>

> Code samples

```javascript
const inputBody = '{}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/webhooks/{provider}',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```

`POST /api/v1/webhooks/{provider}`

> Body parameter

```json
{}
```

<h3 id="receivewebhookevent-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|provider|path|string|true|none|
|body|body|object|true|none|

<h3 id="receivewebhookevent-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Webhook accepted|WebhookIngestResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|
|401|[Unauthorized](https://tools.ietf.org/html/rfc7235#section-3.1)|Invalid signature|Error|

<aside class="success">
This operation does not require authentication
</aside>

