package queryset

import (
	"fmt"
	"log"
	"strings"
	"unicode"

	"github.com/jinzhu/gorm"
)

type method interface {
	GetMethodName() string
	SetReceiverDeclaration(receiverDeclaration string)
	GetReceiverDeclaration() string
	GetArgsDeclaration() string
	GetReturnValuesDeclaration(qsTypeName string) string
	GetBody() string
	GetDoc(methodName string) string
}

// receiverMethod

type receiverMethod struct {
	receiverDeclaration string
}

// GetReceiverDeclaration returns receiver declaration
func (m receiverMethod) GetReceiverDeclaration() string {
	return m.receiverDeclaration
}

func (m *receiverMethod) SetReceiverDeclaration(receiverDeclaration string) {
	m.receiverDeclaration = receiverDeclaration
}

// baseMethod

type baseMethod struct {
	*receiverMethod

	name string
	doc  string
}

func newBaseMethod(name string) baseMethod {
	return baseMethod{
		receiverMethod: &receiverMethod{},
		name:           name,
	}
}

// GetMethodName returns name of method
func (m baseMethod) GetMethodName() string {
	return m.name
}

// GetDoc returns default doc
func (m baseMethod) GetDoc(methodName string) string {
	if m.doc != "" {
		return m.doc
	}

	return fmt.Sprintf(`// %s is an autogenerated method
	// nolint: dupl`, methodName)
}

func (m *baseMethod) setDoc(doc string) {
	m.doc = doc
}

func (m baseMethod) wrapMethod(code string) string {
	const tmpl = `qs.db = qs.db.Scopes(func(d *gorm.DB) *gorm.DB {
      %s})
    return qs`
	return fmt.Sprintf(tmpl, code)
}

// baseQuerySetMethod

type baseQuerySetMethod struct{}

// GetReturnValuesDeclaration gets return values declaration
func (m baseQuerySetMethod) GetReturnValuesDeclaration(qsTypeName string) string {
	return qsTypeName
}

// onFieldMethod

type onFieldMethod struct {
	baseMethod
	fieldName        string
	isFieldNameFirst bool
}

func (m *onFieldMethod) setFieldNameFirst(isFieldNameFirst bool) {
	m.isFieldNameFirst = isFieldNameFirst
}

// GetMethodName returns name of method
func (m onFieldMethod) GetMethodName() string {
	args := []string{m.fieldName, strings.Title(m.name)}
	if !m.isFieldNameFirst {
		args[0], args[1] = args[1], args[0]
	}
	return args[0] + args[1]
}

func newOnFieldMethod(name, fieldName string) onFieldMethod {
	return onFieldMethod{
		baseMethod:       newBaseMethod(name),
		fieldName:        fieldName,
		isFieldNameFirst: true,
	}
}

// oneArgMethod

type oneArgMethod struct {
	argName     string
	argTypeName string
}

func (m oneArgMethod) getArgName() string {
	return m.argName
}

func (m *oneArgMethod) setArgName(argName string) {
	m.argName = argName
}

// GetArgsDeclaration returns declaration of arguments list for func decl
func (m oneArgMethod) GetArgsDeclaration() string {
	return fmt.Sprintf("%s %s", m.getArgName(), m.argTypeName)
}

func newOneArgMethod(argName, argTypeName string) oneArgMethod {
	return oneArgMethod{
		argName:     argName,
		argTypeName: argTypeName,
	}
}

// noArgsMethod

type noArgsMethod struct{}

// GetArgsDeclaration returns declaration of arguments list for func decl
func (m noArgsMethod) GetArgsDeclaration() string {
	return ""
}

type configurableGormMethod struct {
	gormMethodName string
}

func (m *configurableGormMethod) setGormMethodName(name string) {
	m.gormMethodName = name
}

func (m *configurableGormMethod) getGormMethodName() string {
	return m.gormMethodName
}

func newConfigurableGormMethod(name string) configurableGormMethod {
	return configurableGormMethod{gormMethodName: name}
}

// fieldOperationNoArgsMethod

// fieldOperationNoArgsMethod is for unary operations: preload, orderby, etc
type fieldOperationNoArgsMethod struct {
	configurableGormMethod
	onFieldMethod
	transformFieldName bool
	noArgsMethod
	baseQuerySetMethod
}

func (m *fieldOperationNoArgsMethod) setTransformFieldName(v bool) {
	m.transformFieldName = v
}

// GetBody returns method body
func (m fieldOperationNoArgsMethod) GetBody() string {
	fieldName := m.fieldName
	if m.transformFieldName {
		fieldName = gorm.ToDBName(fieldName)
	}
	return m.wrapMethod(fmt.Sprintf(`return d.%s("%s")`, m.getGormMethodName(), fieldName))
}

func newFieldOperationNoArgsMethod(name, fieldName string) fieldOperationNoArgsMethod {
	r := fieldOperationNoArgsMethod{
		onFieldMethod:          newOnFieldMethod(name, fieldName),
		configurableGormMethod: newConfigurableGormMethod(name),
		transformFieldName:     true,
	}
	r.setFieldNameFirst(false) // UserPreload -> PreloadUser
	return r
}

// fieldOperationOneArgMethod

type fieldOperationOneArgMethod struct {
	onFieldMethod
	oneArgMethod
}

// GetBody returns method body
func (m fieldOperationOneArgMethod) GetBody() string {
	return m.wrapMethod(fmt.Sprintf(`return d.%s(%s)`, m.name, m.getArgName()))
}

func lowercaseFirstRune(s string) string {
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func fieldNameToArgName(fieldName string) string {
	if fieldName == "ID" {
		return fieldName
	}

	return lowercaseFirstRune(fieldName)
}

func newFieldOperationOneArgMethod(name, fieldName, argTypeName string) fieldOperationOneArgMethod {
	return fieldOperationOneArgMethod{
		onFieldMethod: newOnFieldMethod(name, fieldName),
		oneArgMethod:  newOneArgMethod(fieldNameToArgName(fieldName), argTypeName),
	}
}

// structOperationOneArgMethod

type structOperationOneArgMethod struct {
	baseMethod
	baseQuerySetMethod
	oneArgMethod
}

// GetBody returns method body
func (m structOperationOneArgMethod) GetBody() string {
	return m.wrapMethod(fmt.Sprintf(`return d.%s(%s)`, m.name, m.getArgName()))
}

func newStructOperationOneArgMethod(name, argTypeName string) structOperationOneArgMethod {
	return structOperationOneArgMethod{
		baseMethod:   newBaseMethod(name),
		oneArgMethod: newOneArgMethod(strings.ToLower(name), argTypeName),
	}
}

// binaryFilterMethod

type binaryFilterMethod struct {
	fieldOperationOneArgMethod
	baseQuerySetMethod
}

func newBinaryFilterMethod(name, fieldName, argTypeName string) binaryFilterMethod {
	return binaryFilterMethod{
		fieldOperationOneArgMethod: newFieldOperationOneArgMethod(name, fieldName, argTypeName),
	}
}

// GetBody returns method's code
func (m binaryFilterMethod) GetBody() string {
	return m.wrapMethod(fmt.Sprintf(`return d.Where("%s %s", %s)`,
		gorm.ToDBName(m.fieldName), m.getWhereCondition(), m.getArgName()))
}

func (m binaryFilterMethod) getWhereCondition() string {
	nameToOp := map[string]string{
		"eq":  "=",
		"ne":  "!=",
		"lt":  "<",
		"lte": "<=",
		"gt":  ">",
		"gte": ">=",
	}
	op := nameToOp[m.name]
	if op == "" {
		log.Fatalf("no operation for filter %q", m.name)
	}

	return fmt.Sprintf("%s ?", op)
}

// unaryFilerMethod

type unaryFilterMethod struct {
	onFieldMethod
	noArgsMethod
	baseQuerySetMethod
	op string
}

func newUnaryFilterMethod(name, fieldName, op string) unaryFilterMethod {
	r := unaryFilterMethod{
		onFieldMethod: newOnFieldMethod(name, fieldName),
		op:            op,
	}
	r.setFieldNameFirst(true)
	return r
}

// GetBody returns method's code
func (m unaryFilterMethod) GetBody() string {
	return m.wrapMethod(fmt.Sprintf(`return d.Where("%s %s")`,
		gorm.ToDBName(m.fieldName), m.op))
}

// unaryFilerMethod

// errorRetMethod

type errorRetMethod struct{}

func (m errorRetMethod) GetReturnValuesDeclaration(string) string {
	return "error"
}

// modelMethod

type modelMethod struct {
	baseMethod
	oneArgMethod
	errorRetMethod
	configurableGormMethod
}

func (m modelMethod) GetBody() string {
	return fmt.Sprintf("return qs.db.%s(%s).Error",
		m.getGormMethodName(), m.getArgName())
}

func newModelMethod(name, gormName, argTypeName string) modelMethod {
	return modelMethod{
		baseMethod:             newBaseMethod(name),
		oneArgMethod:           newOneArgMethod("ret", argTypeName),
		configurableGormMethod: newConfigurableGormMethod(gormName),
	}
}

// dbArgMethod

type dbArgMethod struct {
	oneArgMethod
}

func newDbArgMethod() dbArgMethod {
	return dbArgMethod{
		oneArgMethod: newOneArgMethod("db", "*gorm.DB"),
	}
}

// createMetod

type createMethod struct {
	baseMethod
	dbArgMethod
	errorRetMethod
	structTypeName string
}

func (m createMethod) GetBody() string {
	const tmpl = `if err := db.Create(o).Error; err != nil {
			return fmt.Errorf("can't create %s %%v: %%s", o, err)
		}
		return nil`
	return fmt.Sprintf(tmpl, m.structTypeName)
}

func newCreateMethod(structTypeName string) createMethod {
	r := createMethod{
		baseMethod:     newBaseMethod("Create"),
		dbArgMethod:    newDbArgMethod(),
		structTypeName: structTypeName,
	}
	r.SetReceiverDeclaration(fmt.Sprintf("o *%s", structTypeName))
	return r
}

// Concrete methods

func newPreloadMethod(fieldName string) fieldOperationNoArgsMethod {
	r := newFieldOperationNoArgsMethod("Preload", fieldName)
	r.setTransformFieldName(false)
	return r
}

func newOrderByMethod(fieldName string) fieldOperationNoArgsMethod {
	r := newFieldOperationNoArgsMethod("OrderBy", fieldName)
	r.setGormMethodName("Order")
	return r
}

func newLimitMethod() structOperationOneArgMethod {
	return newStructOperationOneArgMethod("Limit", "int")
}

func newAllMethod(structName string) modelMethod {
	return newModelMethod("All", "Find", fmt.Sprintf("*[]%s", structName))
}

func newOneMethod(structName string) modelMethod {
	r := newModelMethod("One", "First", fmt.Sprintf("*%s", structName))
	const doc = `// One is used to retrieve one result. It returns gorm.ErrRecordNotFound
	// if nothing was fetched`
	r.setDoc(doc)
	return r
}

func newIsNullMethod(fieldName string) unaryFilterMethod {
	return newUnaryFilterMethod("IsNull", fieldName, "IS NULL")
}
