package token

// LookupKeyword returns the hard-keyword or literal kind for text, or
// Identifier when text is not reserved.
func LookupKeyword(text string) Kind {
	switch text {
	case "type":
		return KeywordType
	case "enum":
		return KeywordEnum
	case "module":
		return KeywordModule
	case "capability":
		return KeywordCapability
	case "resource":
		return KeywordResource
	case "variant":
		return KeywordVariant
	case "environment":
		return KeywordEnvironment
	case "transform":
		return KeywordTransform
	case "policy":
		return KeywordPolicy
	case "test":
		return KeywordTest
	case "use":
		return KeywordUse
	case "apply":
		return KeywordApply
	case "enable":
		return KeywordEnable
	case "select":
		return KeywordSelect
	case "where":
		return KeywordWhere
	case "resolve":
		return KeywordResolve
	case "require":
		return KeywordRequire
	case "deny":
		return KeywordDeny
	case "warn":
		return KeywordWarn
	case "export":
		return KeywordExport
	case "protected":
		return KeywordProtected
	case "extension":
		return KeywordExtension
	case "for":
		return KeywordFor
	case "in":
		return KeywordIn
	case "when":
		return KeywordWhen
	case "as":
		return KeywordAs
	case "true":
		return True
	case "false":
		return False
	case "null":
		return Null
	default:
		return Identifier
	}
}
