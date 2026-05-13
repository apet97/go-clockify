Get all invoices on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

query Parameters
page	
integer <int32>
Default: 1
Example: page=1
Page number.

page-size	
integer <int32> >= 1
Default: 50
Example: page-size=50
Page size.

statuses	
string
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Example: statuses=UNSENT&statuses=PAID
If provided, you'll get a filtered result of invoices that matches the provided string in the user ID linked to the expense.

sort-column	
string
Enum: "ID" "CLIENT" "DUE_ON" "ISSUE_DATE" "AMOUNT" "BALANCE"
Example: sort-column=CLIENT
Valid column name as sorting criteria. Default: ID

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Sort order. Default: ASCENDING

Responses
200 OK
Response Schema: application/json
invoices	
Array of objects (InvoiceDtoV1)
Default: "##default"
Represents a list of invoices.

Array 
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

total	
integer <int64>
Represents the total invoice count.


get
/v1/workspaces/{workspaceId}/invoices
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices
Response samples
200
Content type
application/json

Copy
{
"invoices": "##default",
"total": 100
}
Add an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: application/json
required
clientId
required
string non-empty
Default: "##default"
Represents a client identifier across the system.

currency
required
string non-empty
Default: "##default"
Represents the currency used by the invoice.

dueDate
required
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

issuedDate
required
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

number
required
string non-empty
Default: "##default"
Represents an invoice number.

timeViewMode	
string
Enum: "TIME_SENSITIVE_VIEW" "AGGREGATED_TIME_VIEW"
Responses
201 Created
Response Schema: application/json
billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

clientId	
string
Default: "##default"
Represents client identifier across the system.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

number	
string
Default: "##default"
Represents an invoice number.


post
/v1/workspaces/{workspaceId}/invoices
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices
Request samples
Payload
Content type
application/json

Copy
"##default"
Response samples
201
Content type
application/json

Copy
{
"billFrom": "Business X",
"clientId": "34p687e29ae1f428e7ebe562",
"currency": "USD",
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"number": "202306121129"
}
Filter out invoices
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: application/json
required
clients	
object (ContainsArchivedFilterRequest)
Default: "##default"
Represents a project filter for imported items.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN" "CONTAINS_ONLY"
Filter type.

ids	
Array of strings unique
Default: "##default"
Represents a list of filter identifiers.

status	
string
Default: "##default"
Enum: "ACTIVE" "ARCHIVED" "ALL"
Filters entities by status.

companies	
object (BaseFilterRequest)
Default: "##default"
Represents a company filter object. If provided, you'll get a filtered list of invoices that matches the specified company filter.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN" "CONTAINS_ONLY"
Filter type.

ids	
Array of strings unique
Default: "##default"
Represents a list of filter identifiers.

exactAmount	
integer <int64>
Represents an invoice amount. If provided, you'll get a filtered list of invoices that has the equal amount as specified.

exactBalance	
integer <int64>
Represents an invoice balance. If provided, you'll get a filtered list of invoices that has the equal balance as specified.

greaterThanAmount	
integer <int64>
Represents an invoice amount. If provided, you'll get a filtered list of invoices that has amount greater than specified.

greaterThanBalance	
integer <int64>
Represents an invoice balance. If provided, you'll get a filtered list of invoices that has balance greater than specified.

invoiceNumber	
string
Default: "##default"
If provided, you'll get a filtered list of invoices that contain the provided string in their invoice number.

issueDate	
object (TimeRangeRequestDtoV1)
Default: "##default"
Represents a time range object. If provided, you'll get a filtered list of invoices that has issue date within the time range specified.

issue-date-end	
string
Default: "##default"
Represents a date in yyyy-MM-dd format. This is the lower bound of the time range.

issue-date-start	
string
Default: "##default"
Represents a date in yyyy-MM-dd format. This is the lower bound of the time range.

lessThanAmount	
integer <int64>
Represents an invoice amount. If provided, you'll get a filtered list of invoices that has amount less than specified.

lessThanBalance	
integer <int64>
Represents an invoice balance. If provided, you'll get a filtered list of invoices that has balance less than specified.

page	
integer <int32>
Default: 1
Page number.

pageSize	
integer <int32>
Default: 50
Page size.

sortColumn	
string
Default: "##default"
Enum: "ID" "CLIENT" "DUE_ON" "ISSUE_DATE" "AMOUNT" "BALANCE"
Represents the column name to be used as sorting criteria.

sortOrder	
string
Default: "##default"
Enum: "ASCENDING" "DESCENDING"
Represents the sorting order.

statuses	
Array of strings
Default: "##default"
Items Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents a list of invoice statuses. If provided, you'll get a filtered list of invoices that matches any of the invoice status provided.

strictSearch	
boolean
Default: false
Flag to toggle on/off strict search mode. When set to true, search by invoice number only will return invoices whose number exactly matches the string value given for the 'invoiceNumber' parameter. When set to false, results will also include invoices whose number contain the string value, but could be longer than the string value itself. For example, if there is an invoice with the number '123456', and the search value is '123', setting strict-name-search to true will not return that invoice in the results, whereas setting it to false will.

Responses
200 OK
Response Schema: application/json
invoices	
Array of objects (InvoiceInfoV1)
Default: "##default"
Represents a list of invoice info.

Array 
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom an invoice is billed from.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

daysOverdue	
integer <int64>
Represents the number of days an invoice is overdue.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.

One of object
DISCOUNT	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX_2	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
total	
integer <int64>
Represents the total invoice count.


post
/v1/workspaces/{workspaceId}/invoices/info
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/info
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"clients": "##default",
"companies": "##default",
"exactAmount": 1000,
"exactBalance": 1000,
"greaterThanAmount": 500,
"greaterThanBalance": 500,
"invoiceNumber": "Invoice-01",
"issueDate": "##default",
"lessThanAmount": 500,
"lessThanBalance": 500,
"page": 1,
"pageSize": 50,
"sortColumn": "ID",
"sortOrder": "ASCENDING",
"statuses": [
"SENT",
"PAID",
"PARTIALLY_PAID"
],
"strictSearch": false
}
Response samples
200
Content type
application/json

Copy
{
"invoices": "##default",
"total": 100
}
Get an invoice in another language
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Responses
200 OK
Response Schema: application/json
defaults	
object (InvoiceDefaultSettingsDto)
Default: "##default"
Represents an invoice default settings object.

companyId	
string
Default: "##default"
Represents company identifier across the system.

defaultImportExpenseItemTypeId	
string
Default: "##default"
Represents item type identifier across the system.

defaultImportTimeItemTypeId	
string
Default: "##default"
Represents item type identifier across the system.

dueDays	
integer <int32>
Represents an invoice number of due days.

itemTypeId	
string
Default: "##default"
Represents item type identifier across the system.

notes	
string
Default: "##default"
Represents an invoice note.

subject	
string
Default: "##default"
Represents an invoice subject.

tax	
integer <int64>
Deprecated
tax2	
integer <int64>
Deprecated
tax2Percent	
number <double>
Represents a tax amount in percentage.

taxPercent	
number <double>
Represents a tax amount in percentage.

taxType	
string
Default: "##default"
Enum: "COMPOUND" "SIMPLE" "NONE"
Represents a tax type.

exportFields	
object (InvoiceExportFields)
Default: "##default"
Represents an invoice export fields object.

itemType	
boolean
quantity	
boolean
rtl	
boolean
tax	
boolean
tax2	
boolean
unitPrice	
boolean
labels	
object (LabelsCustomization)
Default: "##default"
Represents a label customization object.

amount	
string
Default: "##default"
Represents invoice amount.

billFrom	
string
Default: "##default"
Represents a string an invoice is billed from.

billTo	
string
Default: "##default"
Represents a string an invoice is billed to.

description	
string
Default: "##default"
Represents a description of an invoice.

discount	
string
Default: "##default"
Represents invoice discount amount.

dueDate	
string
Default: "##default"
Represents a due date in yyyy-MM-dd format.

issueDate	
string
Default: "##default"
Represents an issue date in yyyy-MM-dd format.

itemType	
string
Default: "##default"
Represents an item type.

notes	
string
Default: "##default"
Represents notes for an invoice.

paid	
string
Default: "##default"
Represents invoice paid amount.

quantity	
string
Default: "##default"
Represents quantity.

subtotal	
string
Default: "##default"
Represents invoice subtotal.

tax	
string
Default: "##default"
Represents invoice tax amount.

tax2	
string
Default: "##default"
Represents invoice tax amount.

total	
string
Default: "##default"
Represents invoice total amount.

totalAmount	
string
Default: "##default"
Represents invoice total amount.

unitPrice	
string
Default: "##default"
Represents unit price.


get
/v1/workspaces/{workspaceId}/invoices/settings
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/settings
Response samples
200
Content type
application/json

Copy
{
"defaults": "##default",
"exportFields": "##default",
"labels": "##default"
}
Change an invoice language
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: application/json
required
defaults	
object (InvoiceDefaultSettingsRequestV1)
Default: "##default"
Represents an invoice default settings object.

companyId	
string
Default: "##default"
Represents company identifier across the system.

dueDays	
integer <int32>
Represents an invoice number of due days.

itemTypeId	
string
Default: "##default"
Represents item type identifier across the system.

notes
required
string
Default: "##default"
Represents an invoice note.

subject
required
string
Default: "##default"
Represents an invoice subject.

tax2Percent	
number <double>
Represents a tax amount in percentage.

taxPercent	
number <double>
Represents a tax amount in percentage.

taxType	
string
Default: "##default"
Enum: "COMPOUND" "SIMPLE" "NONE"
Represents a tax type.

exportFields	
object (InvoiceExportFieldsRequest)
Default: "##default"
Represents an invoice export fields object.

itemType	
boolean
Default: false
Indicates whether to export item type.

quantity	
boolean
Default: false
Indicates whether to export quantity.

rtl	
boolean
Default: false
Indicates whether to export RTL.

tax	
boolean
Default: false
Indicates whether to export tax.

tax2	
boolean
Default: false
Indicates whether to export tax2.

unitPrice	
boolean
Default: false
Indicates whether to export unit price.

labels
required
object (LabelsCustomizationRequest)
Default: "##default"
Represents a label customization object.

amount
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice amount label.

billFrom
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice bill from label.

billTo
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice bill to label.

description
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice description label.

discount
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice discount amount label.

dueDate
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice due date label.

issueDate
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice issue date label.

itemType
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice item type label.

notes
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice notes label.

paid
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice paid amount label.

quantity
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice quantity label.

subtotal
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice subtotal label.

tax
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice tax amount label.

tax2
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice tax 2 amount label.

total
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice total amount label.

totalAmountDue
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice total amount due label.

unitPrice
required
string [ 0 .. 20 ] characters
Default: "##default"
Represents invoice unit price label.

Responses
200 OK

put
/v1/workspaces/{workspaceId}/invoices/settings
Request samples
Payload
Content type
application/json

Copy
{
"defaults": "##default",
"exportFields": "##default",
"labels": "##default"
}
Delete an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 78a687e29ae1f428e7ebe303
Represents an invoice identifier across the system.

Responses
200 OK

delete
/v1/workspaces/{workspaceId}/invoices/{invoiceId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}
Get an invoice by ID
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 83q687e29ae1f428e7ebe195
Represents an invoice identifier across the system.

Responses
200 OK
Response Schema: application/json
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

calculationType	
object (CalculationType)
Default: "##default"
Represents an enum if tax is calculated as item based or invoice based.

One of object
INVOICE_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
ITEM_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
value	
string
clientAddress	
string
Default: "##default"
Represents client address.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

companyId	
string
Default: "##default"
Represents company identifier across the system.

containsImportedExpenses	
boolean
Default: false
Indicates whether invoice contains imported expenses.

containsImportedTimes	
boolean
Default: false
Indicates whether invoice contains imported items.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

discount	
number <double>
Represents an invoice discount amount as double.

discountAmount	
integer <int64>
Represents an invoice discount amount as long.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

items	
Array of objects (InvoiceItemDto)
Default: "##default"
Represents a list of invoice item datatransfer objects.

Array 
amount	
integer <int64>
Represents item amount.

applyTaxes	
object (ApplyTaxes)
Default: "##default"
Represents item applyTaxes type.

One of object
NONE	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
value	
string
description	
string
Default: "##default"
Represents an invoice item description.

expenseIds	
Array of strings
Default: "##default"
Represents a list of imported expense ids.

importType	
string
Default: "##default"
Enum: "NOT_IMPORTED" "TIME_ENTRY_IMPORT" "EXPENSE_IMPORT"
Represents the invoice item import type.

itemType	
string
Default: "##default"
Represents item type.

order	
integer <int32>
Represents an integer.

quantity	
integer <int64>
Represents item quantity.

timeEntryIds	
Array of strings
Default: "##default"
Represents a list of imported time entry ids.

unitPrice	
integer <int64>
Represents item unit price.

note	
string
Default: "##default"
Represents an invoice note.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

subject	
string
Default: "##default"
Represents an invoice subject.

subtotal	
integer <int64>
Represents an invoice subtotal as long.

tax	
number <double>
Represents an invoice tax amount as double.

tax2	
number <double>
Represents an invoice tax amount as double.

tax2Amount	
integer <int64>
Represents an invoice tax amount as long.

taxAmount	
integer <int64>
Represents an invoice tax amount as long.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

One of object
COMPOUND	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
NONE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
SIMPLE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
value	
string
userId	
string
Default: "##default"
Represents user identifier across the system.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.

One of object
DISCOUNT	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX_2	
string
Enum: "TAX" "TAX_2" "DISCOUNT"

get
/v1/workspaces/{workspaceId}/invoices/{invoiceId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"amount": 100,
"balance": 50,
"billFrom": "Business X",
"calculationType": "INVOICE_BASED",
"clientAddress": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"clientId": "98h687e29ae1f428e7ebe707",
"clientName": "Client X",
"companyId": "04g687e29ae1f428e7ebe123",
"containsImportedExpenses": false,
"containsImportedTimes": false,
"currency": "USD",
"discount": 10.5,
"discountAmount": 11,
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"items": "##default",
"note": "This is a sample note for this invoice.",
"number": "202306121129",
"paid": 50,
"status": "PAID",
"subject": "January salary",
"subtotal": 5000,
"tax": 1.5,
"tax2": 0,
"tax2Amount": 0,
"taxAmount": 1,
"taxType": "SIMPLE",
"userId": "12t687e29ae1f428e7ebe202",
"visibleZeroFields": [
"TAX",
"TAX_2",
"DISCOUNT"
]
}
Update an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 78a687e29ae1f428e7ebe303
Represents an invoice identifier across the system.

Request Body schema: application/json
required
clientId	
string
Default: "##default"
Represents client identifier across the system.

companyId	
string
Default: "##default"
Represents company identifier across the system.

currency
required
string [ 1 .. 100 ] characters
Default: "##default"
Represents the currency used by the invoice.

discountPercent
required
number <double>
Represents an invoice discount percent as double.

dueDate
required
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

issuedDate
required
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

note	
string
Default: "##default"
Represents an invoice note.

number
required
string non-empty
Default: "##default"
Represents an invoice number.

subject	
string
Default: "##default"
Represents an invoice subject.

tax2Percent
required
number <double>
Represents an invoice tax 2 percent as double.

taxPercent
required
number <double>
Represents an invoice tax percent as double.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

One of object
COMPOUND	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
NONE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
SIMPLE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
value	
string
visibleZeroFields	
string
Default: "##default"
Enum: "TAX" "TAX_2" "DISCOUNT"
Represents a list of zero value invoice fields that will be visible.

Responses
200 OK
Response Schema: application/json
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

calculationType	
object (CalculationType)
Default: "##default"
Represents an enum if tax is calculated as item based or invoice based.

One of object
INVOICE_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
ITEM_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
value	
string
clientAddress	
string
Default: "##default"
Represents client address.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

companyId	
string
Default: "##default"
Represents company identifier across the system.

containsImportedExpenses	
boolean
Default: false
Indicates whether invoice contains imported expenses.

containsImportedTimes	
boolean
Default: false
Indicates whether invoice contains imported items.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

discount	
number <double>
Represents an invoice discount amount as double.

discountAmount	
integer <int64>
Represents an invoice discount amount as long.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

items	
Array of objects (InvoiceItemDto)
Default: "##default"
Represents a list of invoice item datatransfer objects.

Array 
amount	
integer <int64>
Represents item amount.

applyTaxes	
object (ApplyTaxes)
Default: "##default"
Represents item applyTaxes type.

One of object
NONE	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
value	
string
description	
string
Default: "##default"
Represents an invoice item description.

expenseIds	
Array of strings
Default: "##default"
Represents a list of imported expense ids.

importType	
string
Default: "##default"
Enum: "NOT_IMPORTED" "TIME_ENTRY_IMPORT" "EXPENSE_IMPORT"
Represents the invoice item import type.

itemType	
string
Default: "##default"
Represents item type.

order	
integer <int32>
Represents an integer.

quantity	
integer <int64>
Represents item quantity.

timeEntryIds	
Array of strings
Default: "##default"
Represents a list of imported time entry ids.

unitPrice	
integer <int64>
Represents item unit price.

note	
string
Default: "##default"
Represents an invoice note.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

subject	
string
Default: "##default"
Represents an invoice subject.

subtotal	
integer <int64>
Represents an invoice subtotal as long.

tax	
number <double>
Represents an invoice tax amount as double.

tax2	
number <double>
Represents an invoice tax amount as double.

tax2Amount	
integer <int64>
Represents an invoice tax amount as long.

taxAmount	
integer <int64>
Represents an invoice tax amount as long.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

One of object
COMPOUND	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
NONE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
SIMPLE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
value	
string
userId	
string
Default: "##default"
Represents user identifier across the system.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.

One of object
DISCOUNT	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX_2	
string
Enum: "TAX" "TAX_2" "DISCOUNT"

put
/v1/workspaces/{workspaceId}/invoices/{invoiceId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}
Request samples
Payload
Content type
application/json

Copy
"##default"
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"amount": 100,
"balance": 50,
"billFrom": "Business X",
"calculationType": "INVOICE_BASED",
"clientAddress": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"clientId": "98h687e29ae1f428e7ebe707",
"clientName": "Client X",
"companyId": "04g687e29ae1f428e7ebe123",
"containsImportedExpenses": false,
"containsImportedTimes": false,
"currency": "USD",
"discount": 10.5,
"discountAmount": 11,
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"items": "##default",
"note": "This is a sample note for this invoice.",
"number": "202306121129",
"paid": 50,
"status": "PAID",
"subject": "January salary",
"subtotal": 5000,
"tax": 1.5,
"tax2": 0,
"tax2Amount": 0,
"taxAmount": 1,
"taxType": "SIMPLE",
"userId": "12t687e29ae1f428e7ebe202",
"visibleZeroFields": [
"TAX",
"TAX_2",
"DISCOUNT"
]
}
Duplicate an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
invoiceId
required
string
Default: "##default"
Example: 78a687e29ae1f428e7ebe303
Represents an invoice identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Responses
201 Created
Response Schema: application/json
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

calculationType	
object (CalculationType)
Default: "##default"
Represents an enum if tax is calculated as item based or invoice based.

One of object
INVOICE_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
ITEM_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
value	
string
clientAddress	
string
Default: "##default"
Represents client address.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

companyId	
string
Default: "##default"
Represents company identifier across the system.

containsImportedExpenses	
boolean
Default: false
Indicates whether invoice contains imported expenses.

containsImportedTimes	
boolean
Default: false
Indicates whether invoice contains imported items.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

discount	
number <double>
Represents an invoice discount amount as double.

discountAmount	
integer <int64>
Represents an invoice discount amount as long.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

items	
Array of objects (InvoiceItemDto)
Default: "##default"
Represents a list of invoice item datatransfer objects.

Array 
amount	
integer <int64>
Represents item amount.

applyTaxes	
object (ApplyTaxes)
Default: "##default"
Represents item applyTaxes type.

One of object
NONE	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
value	
string
description	
string
Default: "##default"
Represents an invoice item description.

expenseIds	
Array of strings
Default: "##default"
Represents a list of imported expense ids.

importType	
string
Default: "##default"
Enum: "NOT_IMPORTED" "TIME_ENTRY_IMPORT" "EXPENSE_IMPORT"
Represents the invoice item import type.

itemType	
string
Default: "##default"
Represents item type.

order	
integer <int32>
Represents an integer.

quantity	
integer <int64>
Represents item quantity.

timeEntryIds	
Array of strings
Default: "##default"
Represents a list of imported time entry ids.

unitPrice	
integer <int64>
Represents item unit price.

note	
string
Default: "##default"
Represents an invoice note.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

subject	
string
Default: "##default"
Represents an invoice subject.

subtotal	
integer <int64>
Represents an invoice subtotal as long.

tax	
number <double>
Represents an invoice tax amount as double.

tax2	
number <double>
Represents an invoice tax amount as double.

tax2Amount	
integer <int64>
Represents an invoice tax amount as long.

taxAmount	
integer <int64>
Represents an invoice tax amount as long.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

One of object
COMPOUND	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
NONE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
SIMPLE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
value	
string
userId	
string
Default: "##default"
Represents user identifier across the system.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.

One of object
DISCOUNT	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX_2	
string
Enum: "TAX" "TAX_2" "DISCOUNT"

post
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/duplicate
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/duplicate
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
{
"amount": 100,
"balance": 50,
"billFrom": "Business X",
"calculationType": "INVOICE_BASED",
"clientAddress": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"clientId": "98h687e29ae1f428e7ebe707",
"clientName": "Client X",
"companyId": "04g687e29ae1f428e7ebe123",
"containsImportedExpenses": false,
"containsImportedTimes": false,
"currency": "USD",
"discount": 10.5,
"discountAmount": 11,
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"items": "##default",
"note": "This is a sample note for this invoice.",
"number": "202306121129",
"paid": 50,
"status": "PAID",
"subject": "January salary",
"subtotal": 5000,
"tax": 1.5,
"tax2": 0,
"tax2Amount": 0,
"taxAmount": 1,
"taxType": "SIMPLE",
"userId": "12t687e29ae1f428e7ebe202",
"visibleZeroFields": [
"TAX",
"TAX_2",
"DISCOUNT"
]
}
Export an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
invoiceId
required
string
Default: "##default"
Example: 78a687e29ae1f428e7ebe303
Represents an invoice identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

query Parameters
userLocale
required
string
Default: "##default"
Example: userLocale=en
Represents a locale.

Responses
200 OK
Response Schema: */*
string <byte>

get
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/export
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/export
Add item to an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 83q687e29ae1f428e7ebe195
Represents an invoice identifier across the system.

Request Body schema: application/json
required
applyTaxes
required
string
Default: "##default"
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
Represents taxes applied to the invoice item. Applies only when the specified taxes are active on the invoice.

description
required
string
Default: "##default"
Represents an invoice item description.

itemType
required
string non-empty
Default: "##default"
Represents an item type.

quantity
required
integer <int64>
Represents an item quantity.

unitPrice
required
integer <int64>
Represents an item unit price.

Responses
200 OK
Response Schema: application/json
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

calculationType	
object (CalculationType)
Default: "##default"
Represents an enum if tax is calculated as item based or invoice based.

One of object
INVOICE_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
ITEM_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
value	
string
clientAddress	
string
Default: "##default"
Represents client address.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

companyId	
string
Default: "##default"
Represents company identifier across the system.

containsImportedExpenses	
boolean
Default: false
Indicates whether invoice contains imported expenses.

containsImportedTimes	
boolean
Default: false
Indicates whether invoice contains imported items.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

discount	
number <double>
Represents an invoice discount amount as double.

discountAmount	
integer <int64>
Represents an invoice discount amount as long.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

items	
Array of objects (InvoiceItemDto)
Default: "##default"
Represents a list of invoice item datatransfer objects.

Array 
amount	
integer <int64>
Represents item amount.

applyTaxes	
object (ApplyTaxes)
Default: "##default"
Represents item applyTaxes type.

One of object
NONE	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
value	
string
description	
string
Default: "##default"
Represents an invoice item description.

expenseIds	
Array of strings
Default: "##default"
Represents a list of imported expense ids.

importType	
string
Default: "##default"
Enum: "NOT_IMPORTED" "TIME_ENTRY_IMPORT" "EXPENSE_IMPORT"
Represents the invoice item import type.

itemType	
string
Default: "##default"
Represents item type.

order	
integer <int32>
Represents an integer.

quantity	
integer <int64>
Represents item quantity.

timeEntryIds	
Array of strings
Default: "##default"
Represents a list of imported time entry ids.

unitPrice	
integer <int64>
Represents item unit price.

note	
string
Default: "##default"
Represents an invoice note.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

subject	
string
Default: "##default"
Represents an invoice subject.

subtotal	
integer <int64>
Represents an invoice subtotal as long.

tax	
number <double>
Represents an invoice tax amount as double.

tax2	
number <double>
Represents an invoice tax amount as double.

tax2Amount	
integer <int64>
Represents an invoice tax amount as long.

taxAmount	
integer <int64>
Represents an invoice tax amount as long.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

One of object
COMPOUND	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
NONE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
SIMPLE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
value	
string
userId	
string
Default: "##default"
Represents user identifier across the system.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.

One of object
DISCOUNT	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX_2	
string
Enum: "TAX" "TAX_2" "DISCOUNT"

post
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/items
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/items
Request samples
Payload
Content type
application/json

Copy
{
"applyTaxes": "TAX1TAX2",
"description": "This is a description of an invoice item.",
"itemType": "Service",
"quantity": 10000,
"unitPrice": 500
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"amount": 100,
"balance": 50,
"billFrom": "Business X",
"calculationType": "INVOICE_BASED",
"clientAddress": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"clientId": "98h687e29ae1f428e7ebe707",
"clientName": "Client X",
"companyId": "04g687e29ae1f428e7ebe123",
"containsImportedExpenses": false,
"containsImportedTimes": false,
"currency": "USD",
"discount": 10.5,
"discountAmount": 11,
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"items": "##default",
"note": "This is a sample note for this invoice.",
"number": "202306121129",
"paid": 50,
"status": "PAID",
"subject": "January salary",
"subtotal": 5000,
"tax": 1.5,
"tax2": 0,
"tax2Amount": 0,
"taxAmount": 1,
"taxType": "SIMPLE",
"userId": "12t687e29ae1f428e7ebe202",
"visibleZeroFields": [
"TAX",
"TAX_2",
"DISCOUNT"
]
}
Import time entries and expenses to an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 83q687e29ae1f428e7ebe195
Represents an invoice identifier across the system.

Request Body schema: application/json
required
expenseFieldsForDetailedGroup	
Array of strings unique
Default: "NOTE"
Items Enum: "PROJECT" "TASK" "CATEGORY" "NOTE" "DATE" "USER"
Represents a set of expense fields to include when using the DETAILED expense grouping type.

expensesGroupBy	
string
Default: "PROJECT"
Enum: "CATEGORY" "PROJECT" "USER"
Represents a group field when using the GROUPED expense group type.

expensesGroupType	
string
Default: "DETAILED"
Enum: "GROUPED" "DETAILED"
Represents an expense group type.

from
required
string
Default: "##default"
Represents date and time in the yyyy-MM-ddThh:mm:ssZ format.

importExpenses
required
boolean
Default: false
Indicates if billable expenses should be imported alongside time entries.

projectFilter
required
object (ContainsArchivedFilterRequest)
Default: "##default"
Represents a project filter for imported items.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN" "CONTAINS_ONLY"
Filter type.

ids	
Array of strings unique
Default: "##default"
Represents a list of filter identifiers.

status	
string
Default: "##default"
Enum: "ACTIVE" "ARCHIVED" "ALL"
Filters entities by status.

roundTimeEntryDuration	
boolean
Default: false
Indicates if imported time entry durations should be rounded to the nearest 15 minute interval.

timeEntryFieldsForDetailedGroup	
Array of strings unique
Default: "##default"
Items Enum: "PROJECT" "TASK" "TAGS" "DESCRIPTION" "DATE" "USER"
Represents a set of time entry fields to include when using DETAILED time entry grouping type.

timeEntryGroupType
required
string
Default: "##default"
Enum: "SINGLE_ITEM" "GROUPED" "DETAILED"
Represents a time entry group type.

timeEntryPrimaryGroupBy	
string
Default: "##default"
Enum: "USER" "PROJECT" "DATE"
Represents a primary group field when using the GROUPED time entry grouping type.

timeEntrySecondaryGroupBy	
string
Default: "##default"
Enum: "PROJECT" "USER" "TASK" "DATE" "DESCRIPTION" "NONE"
Represents a secondary group field when using the GROUPED time entry grouping type. Should not have the same grouping type as the primary group field.

to
required
string
Default: "##default"
Represents date and time in the yyyy-MM-ddThh:mm:ssZ format.

Responses
200 OK
Response Schema: application/json
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

calculationType	
object (CalculationType)
Default: "##default"
Represents an enum if tax is calculated as item based or invoice based.

One of object
INVOICE_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
ITEM_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
value	
string
clientAddress	
string
Default: "##default"
Represents client address.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

companyId	
string
Default: "##default"
Represents company identifier across the system.

containsImportedExpenses	
boolean
Default: false
Indicates whether invoice contains imported expenses.

containsImportedTimes	
boolean
Default: false
Indicates whether invoice contains imported items.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

discount	
number <double>
Represents an invoice discount amount as double.

discountAmount	
integer <int64>
Represents an invoice discount amount as long.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

items	
Array of objects (InvoiceItemDto)
Default: "##default"
Represents a list of invoice item datatransfer objects.

note	
string
Default: "##default"
Represents an invoice note.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

subject	
string
Default: "##default"
Represents an invoice subject.

subtotal	
integer <int64>
Represents an invoice subtotal as long.

tax	
number <double>
Represents an invoice tax amount as double.

tax2	
number <double>
Represents an invoice tax amount as double.

tax2Amount	
integer <int64>
Represents an invoice tax amount as long.

taxAmount	
integer <int64>
Represents an invoice tax amount as long.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

userId	
string
Default: "##default"
Represents user identifier across the system.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.

One of object
DISCOUNT	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX_2	
string
Enum: "TAX" "TAX_2" "DISCOUNT"

post
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/items/import
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/items/import
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"expenseFieldsForDetailedGroup": [
"NOTE"
],
"expensesGroupBy": "CATEGORY",
"expensesGroupType": "GROUPED",
"from": "2025-06-01T00:00:00Z",
"importExpenses": false,
"projectFilter": "##default",
"roundTimeEntryDuration": false,
"timeEntryFieldsForDetailedGroup": [
"PROJECT",
"DESCRIPTION"
],
"timeEntryGroupType": "GROUPED",
"timeEntryPrimaryGroupBy": "PROJECT",
"timeEntrySecondaryGroupBy": "TASK",
"to": "2025-06-07T00:00:00Z"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"amount": 100,
"balance": 50,
"billFrom": "Business X",
"calculationType": "INVOICE_BASED",
"clientAddress": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"clientId": "98h687e29ae1f428e7ebe707",
"clientName": "Client X",
"companyId": "04g687e29ae1f428e7ebe123",
"containsImportedExpenses": false,
"containsImportedTimes": false,
"currency": "USD",
"discount": 10.5,
"discountAmount": 11,
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"items": "##default",
"note": "This is a sample note for this invoice.",
"number": "202306121129",
"paid": 50,
"status": "PAID",
"subject": "January salary",
"subtotal": 5000,
"tax": 1.5,
"tax2": 0,
"tax2Amount": 0,
"taxAmount": 1,
"taxType": "SIMPLE",
"userId": "12t687e29ae1f428e7ebe202",
"visibleZeroFields": [
"TAX",
"TAX_2",
"DISCOUNT"
]
}
Delete item from an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 83q687e29ae1f428e7ebe195
Represents an invoice identifier across the system.

order
required
integer <int32> >= 1
Example: 3
Represents an invoice item order.

Responses
200 OK
Response Schema: application/json
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

calculationType	
object (CalculationType)
Default: "##default"
Represents an enum if tax is calculated as item based or invoice based.

One of object
INVOICE_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
ITEM_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
value	
string
clientAddress	
string
Default: "##default"
Represents client address.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

companyId	
string
Default: "##default"
Represents company identifier across the system.

containsImportedExpenses	
boolean
Default: false
Indicates whether invoice contains imported expenses.

containsImportedTimes	
boolean
Default: false
Indicates whether invoice contains imported items.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

discount	
number <double>
Represents an invoice discount amount as double.

discountAmount	
integer <int64>
Represents an invoice discount amount as long.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

items	
Array of objects (InvoiceItemDto)
Default: "##default"
Represents a list of invoice item datatransfer objects.

Array 
amount	
integer <int64>
Represents item amount.

applyTaxes	
object (ApplyTaxes)
Default: "##default"
Represents item applyTaxes type.

One of object
NONE	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
value	
string
description	
string
Default: "##default"
Represents an invoice item description.

expenseIds	
Array of strings
Default: "##default"
Represents a list of imported expense ids.

importType	
string
Default: "##default"
Enum: "NOT_IMPORTED" "TIME_ENTRY_IMPORT" "EXPENSE_IMPORT"
Represents the invoice item import type.

itemType	
string
Default: "##default"
Represents item type.

order	
integer <int32>
Represents an integer.

quantity	
integer <int64>
Represents item quantity.

timeEntryIds	
Array of strings
Default: "##default"
Represents a list of imported time entry ids.

unitPrice	
integer <int64>
Represents item unit price.

note	
string
Default: "##default"
Represents an invoice note.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

subject	
string
Default: "##default"
Represents an invoice subject.

subtotal	
integer <int64>
Represents an invoice subtotal as long.

tax	
number <double>
Represents an invoice tax amount as double.

tax2	
number <double>
Represents an invoice tax amount as double.

tax2Amount	
integer <int64>
Represents an invoice tax amount as long.

taxAmount	
integer <int64>
Represents an invoice tax amount as long.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

One of object
COMPOUND	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
NONE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
SIMPLE	
string
Enum: "COMPOUND" "SIMPLE" "NONE"
value	
string
userId	
string
Default: "##default"
Represents user identifier across the system.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.

One of object
DISCOUNT	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX	
string
Enum: "TAX" "TAX_2" "DISCOUNT"
TAX_2	
string
Enum: "TAX" "TAX_2" "DISCOUNT"

delete
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/items/{order}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/items/{order}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"amount": 100,
"balance": 50,
"billFrom": "Business X",
"calculationType": "INVOICE_BASED",
"clientAddress": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"clientId": "98h687e29ae1f428e7ebe707",
"clientName": "Client X",
"companyId": "04g687e29ae1f428e7ebe123",
"containsImportedExpenses": false,
"containsImportedTimes": false,
"currency": "USD",
"discount": 10.5,
"discountAmount": 11,
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"items": "##default",
"note": "This is a sample note for this invoice.",
"number": "202306121129",
"paid": 50,
"status": "PAID",
"subject": "January salary",
"subtotal": 5000,
"tax": 1.5,
"tax2": 0,
"tax2Amount": 0,
"taxAmount": 1,
"taxType": "SIMPLE",
"userId": "12t687e29ae1f428e7ebe202",
"visibleZeroFields": [
"TAX",
"TAX_2",
"DISCOUNT"
]
}
Get payments for an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 78a687e29ae1f428e7ebe303
Represents an invoice identifier across the system.

query Parameters
page	
integer <int32>
Default: 1
Example: page=1
Page number.

page-size	
integer <int32> >= 1
Default: 50
Example: page-size=50
Page size.

Responses
200 OK
Response Schema: application/json
Array 
amount	
integer <int64>
Represents an invoice payment amount as long.

author	
string
Default: "##default"
Represents an invoice payment author.

date	
string <date-time>
Represents an invoice payment date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice payment identifier across the system.

note	
string
Default: "##default"
Represents an invoice payment note.


get
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/payments
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/payments
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"amount": 100,
"author": "John Doe",
"date": "2021-01-01T12:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"note": "This is a sample note for this invoice payment."
}
]
Add payment to an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 78a687e29ae1f428e7ebe303
Represents an invoice identifier across the system.

Request Body schema: application/json
required
amount	
integer <int64> >= 1
Represents an invoice payment amount as long.

note	
string [ 0 .. 1000 ] characters
Default: "##default"
Represents an invoice payment note.

paymentDate	
string
Default: "##default"
Represents an invoice payment date in yyyy-MM-ddThh:mm:ssZ format.

Responses
201 Created
Response Schema: application/json
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

calculationType	
object (CalculationType)
Default: "##default"
Represents an enum if tax is calculated as item based or invoice based.

One of object
INVOICE_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
ITEM_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
value	
string
clientAddress	
string
Default: "##default"
Represents client address.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

companyId	
string
Default: "##default"
Represents company identifier across the system.

containsImportedExpenses	
boolean
Default: false
Indicates whether invoice contains imported expenses.

containsImportedTimes	
boolean
Default: false
Indicates whether invoice contains imported items.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

discount	
number <double>
Represents an invoice discount amount as double.

discountAmount	
integer <int64>
Represents an invoice discount amount as long.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

items	
Array of objects (InvoiceItemDto)
Default: "##default"
Represents a list of invoice item datatransfer objects.

note	
string
Default: "##default"
Represents an invoice note.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

subject	
string
Default: "##default"
Represents an invoice subject.

subtotal	
integer <int64>
Represents an invoice subtotal as long.

tax	
number <double>
Represents an invoice tax amount as double.

tax2	
number <double>
Represents an invoice tax amount as double.

tax2Amount	
integer <int64>
Represents an invoice tax amount as long.

taxAmount	
integer <int64>
Represents an invoice tax amount as long.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

userId	
string
Default: "##default"
Represents user identifier across the system.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.


post
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/payments
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/payments
Request samples
Payload
Content type
application/json

Copy
{
"amount": 100,
"note": "This is a sample note for this invoice payment.",
"paymentDate": "2021-01-01T12:00:00Z"
}
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
{
"amount": 100,
"balance": 50,
"billFrom": "Business X",
"calculationType": "INVOICE_BASED",
"clientAddress": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"clientId": "98h687e29ae1f428e7ebe707",
"clientName": "Client X",
"companyId": "04g687e29ae1f428e7ebe123",
"containsImportedExpenses": false,
"containsImportedTimes": false,
"currency": "USD",
"discount": 10.5,
"discountAmount": 11,
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"items": "##default",
"note": "This is a sample note for this invoice.",
"number": "202306121129",
"paid": 50,
"status": "PAID",
"subject": "January salary",
"subtotal": 5000,
"tax": 1.5,
"tax2": 0,
"tax2Amount": 0,
"taxAmount": 1,
"taxType": "SIMPLE",
"userId": "12t687e29ae1f428e7ebe202",
"visibleZeroFields": [
"TAX",
"TAX_2",
"DISCOUNT"
]
}
Delete payment from an invoice
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
invoiceId
required
string
Default: "##default"
Example: 78a687e29ae1f428e7ebe303
Represents an invoice identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

paymentId
required
string
Default: "##default"
Example: 56p687e29ae1f428e7ebe456
Represents a payment identifier across the system.

Responses
200 OK
Response Schema: application/json
amount	
integer <int64>
Represents an invoice amount as long.

balance	
integer <int64>
Represents an invoice balance amount as long.

billFrom	
string
Default: "##default"
Represents to whom the invoice should be billed from.

calculationType	
object (CalculationType)
Default: "##default"
Represents an enum if tax is calculated as item based or invoice based.

One of object
INVOICE_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
ITEM_BASED	
string
Enum: "INVOICE_BASED" "ITEM_BASED"
value	
string
clientAddress	
string
Default: "##default"
Represents client address.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name for an invoice.

companyId	
string
Default: "##default"
Represents company identifier across the system.

containsImportedExpenses	
boolean
Default: false
Indicates whether invoice contains imported expenses.

containsImportedTimes	
boolean
Default: false
Indicates whether invoice contains imported items.

currency	
string
Default: "##default"
Represents the currency used by the invoice.

discount	
number <double>
Represents an invoice discount amount as double.

discountAmount	
integer <int64>
Represents an invoice discount amount as long.

dueDate	
string <date-time>
Represents an invoice due date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents invoice identifier across the system.

issuedDate	
string <date-time>
Represents an invoice issued date in yyyy-MM-ddThh:mm:ssZ format.

items	
Array of objects (InvoiceItemDto)
Default: "##default"
Represents a list of invoice item datatransfer objects.

Array 
amount	
integer <int64>
Represents item amount.

applyTaxes	
object (ApplyTaxes)
Default: "##default"
Represents item applyTaxes type.

One of object
NONE	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX1TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
TAX2	
string
Enum: "TAX1" "TAX2" "TAX1TAX2" "NONE"
value	
string
description	
string
Default: "##default"
Represents an invoice item description.

expenseIds	
Array of strings
Default: "##default"
Represents a list of imported expense ids.

importType	
string
Default: "##default"
Enum: "NOT_IMPORTED" "TIME_ENTRY_IMPORT" "EXPENSE_IMPORT"
Represents the invoice item import type.

itemType	
string
Default: "##default"
Represents item type.

order	
integer <int32>
Represents an integer.

quantity	
integer <int64>
Represents item quantity.

timeEntryIds	
Array of strings
Default: "##default"
Represents a list of imported time entry ids.

unitPrice	
integer <int64>
Represents item unit price.

note	
string
Default: "##default"
Represents an invoice note.

number	
string
Default: "##default"
Represents an invoice number.

paid	
integer <int64>
Represents an invoice paid amount as long.

status	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the status of an invoice.

subject	
string
Default: "##default"
Represents an invoice subject.

subtotal	
integer <int64>
Represents an invoice subtotal as long.

tax	
number <double>
Represents an invoice tax amount as double.

tax2	
number <double>
Represents an invoice tax amount as double.

tax2Amount	
integer <int64>
Represents an invoice tax amount as long.

taxAmount	
integer <int64>
Represents an invoice tax amount as long.

taxType	
object (TaxType)
Default: "##default"
Represents an invoice taxation type.

userId	
string
Default: "##default"
Represents user identifier across the system.

visibleZeroFields	
object (VisibleZeroFieldsInvoice)
Default: "##default"
Represents a list of zero value invoice fields that will be visible.


delete
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/payments/{paymentId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/payments/{paymentId}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"amount": 100,
"balance": 50,
"billFrom": "Business X",
"calculationType": "INVOICE_BASED",
"clientAddress": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
"clientId": "98h687e29ae1f428e7ebe707",
"clientName": "Client X",
"companyId": "04g687e29ae1f428e7ebe123",
"containsImportedExpenses": false,
"containsImportedTimes": false,
"currency": "USD",
"discount": 10.5,
"discountAmount": 11,
"dueDate": "2020-06-01T08:00:00Z",
"id": "78a687e29ae1f428e7ebe303",
"issuedDate": "2020-01-01T08:00:00Z",
"items": "##default",
"note": "This is a sample note for this invoice.",
"number": "202306121129",
"paid": 50,
"status": "PAID",
"subject": "January salary",
"subtotal": 5000,
"tax": 1.5,
"tax2": 0,
"tax2Amount": 0,
"taxAmount": 1,
"taxType": "SIMPLE",
"userId": "12t687e29ae1f428e7ebe202",
"visibleZeroFields": [
"TAX",
"TAX_2",
"DISCOUNT"
]
}
Change an invoice status
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

invoiceId
required
string
Default: "##default"
Example: 78a687e29ae1f428e7ebe303
Represents an invoice identifier across the system.

Request Body schema: application/json
required
invoiceStatus	
string
Default: "##default"
Enum: "UNSENT" "SENT" "PAID" "PARTIALLY_PAID" "VOID" "OVERDUE"
Represents the invoice status to be set.

Responses
200 OK

patch
/v1/workspaces/{workspaceId}/invoices/{invoiceId}/status
https://api.clockify.me/api/v1/workspaces/{workspaceId}/invoices/{invoiceId}/status
Request samples
Payload
Content type
application/json

Copy
{
"invoiceStatus": "PAID"
}
