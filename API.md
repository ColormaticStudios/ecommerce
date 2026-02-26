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
  "product_id": 1,
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
  "product_id": 1,
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
      "product_id": 1,
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
      "product_id": 1,
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

## processPayment

<a id="opIdprocessPayment"></a>

> Code samples

```javascript
const inputBody = '{
  "payment_method_id": 0,
  "address_id": 0,
  "payment_method": {
    "cardholder_name": "string",
    "card_number": "string",
    "exp_month": 1,
    "exp_year": 0
  },
  "address": {
    "full_name": "string",
    "line1": "string",
    "line2": "string",
    "city": "string",
    "state": "string",
    "postal_code": "string",
    "country": "string"
  },
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

fetch('http://localhost:3000/api/v1/me/orders/{id}/pay',
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

`POST /api/v1/me/orders/{id}/pay`

> Body parameter

```json
{
  "payment_method_id": 0,
  "address_id": 0,
  "payment_method": {
    "cardholder_name": "string",
    "card_number": "string",
    "exp_month": 1,
    "exp_year": 0
  },
  "address": {
    "full_name": "string",
    "line1": "string",
    "line2": "string",
    "city": "string",
    "state": "string",
    "postal_code": "string",
    "country": "string"
  },
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

<h3 id="processpayment-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|ProcessPaymentRequest|false|none|

<h3 id="processpayment-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Payment result|ProcessPaymentResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

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
  "description": "string",
  "price": 0.1,
  "stock": 0,
  "images": [
    "string"
  ]
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
  "description": "string",
  "price": 0.1,
  "stock": 0,
  "images": [
    "string"
  ]
}
```

<h3 id="createproduct-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|ProductInput|true|none|

<h3 id="createproduct-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|201|[Created](https://tools.ietf.org/html/rfc7231#section-6.3.2)|Created product|Product|

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
  "description": "string",
  "price": 0.1,
  "stock": 0,
  "images": [
    "string"
  ]
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
  "description": "string",
  "price": 0.1,
  "stock": 0,
  "images": [
    "string"
  ]
}
```

<h3 id="updateproduct-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|id|path|integer|true|none|
|body|body|ProductInput|true|none|

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

## getAdminStorefrontSettings

<a id="opIdgetAdminStorefrontSettings"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/storefront',
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

`GET /api/v1/admin/storefront`

<h3 id="getadminstorefrontsettings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Storefront settings|StorefrontSettingsResponse|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## updateStorefrontSettings

<a id="opIdupdateStorefrontSettings"></a>

> Code samples

```javascript
const inputBody = '{
  "settings": {
    "site_title": "string",
    "homepage_sections": [
      {
        "id": "string",
        "type": "hero",
        "enabled": true,
        "hero": {
          "eyebrow": "string",
          "title": "string",
          "subtitle": "string",
          "background_image_url": "string",
          "background_image_media_id": "string",
          "primary_cta": {
            "label": "string",
            "url": "string"
          },
          "secondary_cta": {
            "label": "string",
            "url": "string"
          }
        },
        "product_section": {
          "title": "string",
          "subtitle": "string",
          "source": "manual",
          "query": "string",
          "product_ids": [
            1
          ],
          "sort": "created_at",
          "order": "asc",
          "limit": 1,
          "show_stock": true,
          "show_description": true,
          "image_aspect": "square"
        },
        "promo_cards": [
          {
            "kicker": "string",
            "title": "string",
            "description": "string",
            "image_url": "string",
            "link": {
              "label": "string",
              "url": "string"
            }
          }
        ],
        "promo_card_limit": 1,
        "badges": [
          "string"
        ]
      }
    ],
    "footer": {
      "brand_name": "string",
      "tagline": "string",
      "copyright": "string",
      "columns": [
        {
          "title": "string",
          "links": [
            {
              "label": "string",
              "url": "string"
            }
          ]
        }
      ],
      "social_links": [
        {
          "label": "string",
          "url": "string"
        }
      ],
      "bottom_notice": "string"
    }
  }
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/storefront',
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

`PUT /api/v1/admin/storefront`

> Body parameter

```json
{
  "settings": {
    "site_title": "string",
    "homepage_sections": [
      {
        "id": "string",
        "type": "hero",
        "enabled": true,
        "hero": {
          "eyebrow": "string",
          "title": "string",
          "subtitle": "string",
          "background_image_url": "string",
          "background_image_media_id": "string",
          "primary_cta": {
            "label": "string",
            "url": "string"
          },
          "secondary_cta": {
            "label": "string",
            "url": "string"
          }
        },
        "product_section": {
          "title": "string",
          "subtitle": "string",
          "source": "manual",
          "query": "string",
          "product_ids": [
            1
          ],
          "sort": "created_at",
          "order": "asc",
          "limit": 1,
          "show_stock": true,
          "show_description": true,
          "image_aspect": "square"
        },
        "promo_cards": [
          {
            "kicker": "string",
            "title": "string",
            "description": "string",
            "image_url": "string",
            "link": {
              "label": "string",
              "url": "string"
            }
          }
        ],
        "promo_card_limit": 1,
        "badges": [
          "string"
        ]
      }
    ],
    "footer": {
      "brand_name": "string",
      "tagline": "string",
      "copyright": "string",
      "columns": [
        {
          "title": "string",
          "links": [
            {
              "label": "string",
              "url": "string"
            }
          ]
        }
      ],
      "social_links": [
        {
          "label": "string",
          "url": "string"
        }
      ],
      "bottom_notice": "string"
    }
  }
}
```

<h3 id="updatestorefrontsettings-parameters">Parameters</h3>

|Name|In|Type|Required|Description|
|---|---|---|---|---|
|body|body|StorefrontSettingsRequest|true|none|

<h3 id="updatestorefrontsettings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Updated storefront settings|StorefrontSettingsResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## publishStorefrontSettings

<a id="opIdpublishStorefrontSettings"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/storefront/publish',
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

`POST /api/v1/admin/storefront/publish`

<h3 id="publishstorefrontsettings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Published storefront settings|StorefrontSettingsResponse|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Bad request|Error|

<aside class="warning">
To perform this operation, you must be authenticated by means of one of the following methods:
cookieAuth, bearerAuth
</aside>

## discardStorefrontDraft

<a id="opIddiscardStorefrontDraft"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/admin/storefront/draft',
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

`DELETE /api/v1/admin/storefront/draft`

<h3 id="discardstorefrontdraft-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Storefront settings after draft discard|StorefrontSettingsResponse|

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

<h1 id="ecommerce-api-storefront">storefront</h1>

## getStorefrontSettings

<a id="opIdgetStorefrontSettings"></a>

> Code samples

```javascript

const headers = {
  'Accept':'application/json'
};

fetch('http://localhost:3000/api/v1/storefront',
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

`GET /api/v1/storefront`

<h3 id="getstorefrontsettings-responses">Responses</h3>

|Status|Meaning|Description|Schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Storefront settings|StorefrontSettingsResponse|
|500|[Internal Server Error](https://tools.ietf.org/html/rfc7231#section-6.6.1)|Internal server error|Error|

<aside class="success">
This operation does not require authentication
</aside>

