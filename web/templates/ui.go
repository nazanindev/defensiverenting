package templates

import (
	"fmt"
	"time"
)

// UI chrome strings for the language-routed pages (ADR-007 D2's content
// routes: playbook, jurisdiction hub, topic hub, 404, and the shared footer).
// Statements are translated by the drafting pipeline; this table is for the
// furniture around them, so a Spanish page never wraps Spanish law in English
// chrome. The Spanish register is usted, matching the translated statements.
//
// Lookup rules: a missing language falls back to English; a missing key
// renders the key itself, which is deliberately ugly so it cannot ship
// unnoticed. English-only chrome (/, /search, the reference layer) keeps its
// strings inline in the templates; only strings a Spanish page renders live
// here.
var uiStrings = map[string]map[string]string{
	// Shared header and footer
	"search-placeholder": {"en": "Search by situation…", "es": "Busque su situación…"},
	"search":             {"en": "Search", "es": "Buscar"},
	"footer-disclaimer": {
		"en": "This site provides general information and is not a substitute for a lawyer. Every claim links to its primary source so you can verify it yourself.",
		"es": "Este sitio ofrece información general y no reemplaza a un abogado. Cada afirmación enlaza a su fuente primaria para que usted mismo pueda verificarla.",
	},
	"not-legal-advice": {"en": "Not legal advice.", "es": "Esto no es asesoría legal."},
	"footer-locations": {"en": "All locations", "es": "Todos los lugares"},
	"footer-about":     {"en": "About", "es": "Acerca de"},
	"footer-support":   {"en": "Support this project", "es": "Apoye este proyecto"},
	"footer-editorial": {"en": "Editorial guidance", "es": "Guía editorial"},
	"footer-report":    {"en": "Report a problem", "es": "Reporte un problema"},
	"footer-contact":   {"en": "Contact us", "es": "Contacto"},
	"home":             {"en": "Home", "es": "Inicio"},

	// Playbook page
	"all-topics":     {"en": "← All topics", "es": "← Todos los temas"},
	"nationwide":     {"en": "Nationwide", "es": "Todo el país"},
	"reviewed-by":    {"en": "Last reviewed by", "es": "Última revisión por"},
	"last-verified":  {"en": "Last verified", "es": "Verificado por última vez el"},
	"need-help-now":  {"en": "Need help now?", "es": "¿Necesita ayuda ahora?"},
	"in":             {"en": "in", "es": "en"},
	"help-bar-tail":  {"en": "legal aid, rent assistance, and who to call", "es": "ayuda legal, ayuda con la renta y a quién llamar"},
	"page-disclaimer": {
		"en": "Every statement below links to its primary source. Read the source before relying on this information.",
		"es": "Cada afirmación de abajo enlaza a su fuente primaria. Lea la fuente antes de confiar en esta información.",
	},
	"page-disclaimer-help": {
		"en": "If you need legal help, contact your local legal aid organization.",
		"es": "Si necesita ayuda legal, contacte a la organización de ayuda legal de su área.",
	},
	"sources":          {"en": "Sources", "es": "Fuentes"},
	"sources-checked":  {"en": "Sources checked", "es": "Fuentes verificadas el"},
	"details-checked":  {"en": "Details checked", "es": "Datos verificados el"},
	"rule-depends":     {"en": "This rule depends on where you live.", "es": "Esta regla depende de dónde vive."},
	"rule-everywhere":  {"en": "See the rule in every place we cover.", "es": "Vea la regla en cada lugar que cubrimos."},
	"full-guides":      {"en": "We have full guides on this.", "es": "Tenemos guías completas sobre esto."},
	"see-guides":       {"en": "See our %s guides.", "es": "Vea nuestras guías de %s."},
	"no-statements":    {"en": "No statements available for this playbook yet.", "es": "Aún no hay contenido en esta guía."},
	"report-question":  {"en": "Is something on this page wrong or out of date?", "es": "¿Algo en esta página está mal o desactualizado?"},
	"report-tell-us":   {"en": "Tell us", "es": "Avísenos"},
	"more-nationwide":  {"en": "More nationwide tenant rights guides", "es": "Más guías nacionales de derechos del inquilino"},
	"more-rights-in":   {"en": "More tenant rights in %s", "es": "Más derechos del inquilino en %s"},
	"topic-your-state": {"en": "%s in your state or city", "es": "%s en su estado o ciudad"},
	"topic-elsewhere":  {"en": "%s in other cities", "es": "%s en otras ciudades"},
	"all-cities-topic": {"en": "All cities for this topic →", "es": "Todas las ciudades para este tema →"},

	// Jurisdiction hub
	"whats-your-situation": {"en": "What’s your situation?", "es": "¿Cuál es su situación?"},
	"hub-lede": {
		"en": "Describe what is happening and get a step by step tenant rights playbook. Every statement is backed by a primary source.",
		"es": "Estas guías explican sus derechos como inquilino paso a paso. Cada afirmación está respaldada por una fuente primaria.",
	},
	"hub-search-placeholder": {"en": "e.g. heat stopped working…", "es": "por ejemplo: la calefacción no funciona…"},
	"nationwide-guides":      {"en": "Nationwide guides", "es": "Guías nacionales"},
	"statewide-rules":        {"en": "%s statewide rules", "es": "Reglas estatales de %s"},
	"pick-a-topic":           {"en": "Or pick a topic", "es": "Elija un tema"},
	"cities-in":              {"en": "Cities in %s", "es": "Ciudades en %s"},
	"ordinances-stack": {
		"en": "Local ordinances stack on top of %s law, so start with your city where we cover it.",
		"es": "Las reglas locales se suman a la ley de %s. Si cubrimos su ciudad, empiece por ahí.",
	},
	"no-playbooks-yet":  {"en": "No playbooks available for %s yet.", "es": "Aún no hay guías para %s."},
	"see-all-locations": {"en": "See all locations →", "es": "Vea todos los lugares →"},

	// Topic hub
	"topic-hub-lede": {
		"en": "Tenant rights on this topic vary by city and state. Pick your city for a step-by-step guide. Every claim cites the law it comes from.",
		"es": "Los derechos del inquilino en este tema cambian según la ciudad y el estado. Elija su ciudad para ver una guía paso a paso. Cada afirmación cita la ley de donde viene.",
	},
	"choose-your-city": {"en": "Choose your city", "es": "Elija su ciudad"},
	"covered-count":    {"en": "(%d covered)", "es": "(%d cubiertas)"},
	"other-group":      {"en": "Other", "es": "Otros"},
	"dont-see-city": {
		"en": "Don’t see your city? We add new cities regularly. Statewide rules often still apply; check the guide for the nearest covered city in your state to see which laws are cited.",
		"es": "¿No ve su ciudad? Agregamos ciudades nuevas con frecuencia. Las reglas estatales muchas veces aplican de todos modos; revise la guía de la ciudad cubierta más cercana en su estado para ver qué leyes se citan.",
	},
	"statewide-guides": {"en": "Statewide guides", "es": "Guías estatales"},
	"national-guide":   {"en": "Nationwide guide", "es": "Guía nacional"},
	"national-applies": {
		"en": "This guide explains the rules that apply in every state. Your state and city can add more protections on top.",
		"es": "Esta guía explica las reglas que aplican en todos los estados. Su estado y su ciudad pueden sumar más protecciones.",
	},

	// 404
	"nf-no-guide-for":    {"en": "No %s guide for %s yet", "es": "Aún no hay guía de %s para %s"},
	"nf-other-guides":    {"en": "We have other guides for %s, but not this one yet.", "es": "Tenemos otras guías para %s, pero esta todavía no."},
	"nf-no-topic-guides": {"en": "No %s guides yet", "es": "Aún no hay guías de %s"},
	"nf-none-published":  {"en": "No guides are published for this topic yet.", "es": "Aún no hay guías publicadas para este tema."},
	"nf-uncovered":       {"en": "We don’t cover this place yet", "es": "Aún no cubrimos este lugar"},
	"nf-one-at-a-time":   {"en": "RenterLaw adds new places one at a time.", "es": "RenterLaw agrega lugares nuevos uno por uno."},
	"nf-not-found":       {"en": "We can’t find that page", "es": "No encontramos esa página"},
	"nf-moved":           {"en": "The link may be wrong, or the page may have moved.", "es": "El enlace puede estar mal, o la página pudo haber cambiado de lugar."},
	"nf-nearest-covers":  {"en": "The %s guide covers this", "es": "La guía de %s cubre esto"},
	"nf-nearest-applies": {"en": "%s law applies in %s. Start there until we write the %s guide.", "es": "La ley de %s aplica en %s. Empiece ahí mientras escribimos la guía de %s."},
	"nf-topic-in":        {"en": "%s in %s", "es": "%s en %s"},
	"nf-where-instead":   {"en": "Where to go instead", "es": "A dónde ir mientras tanto"},
	"nf-everything-for":  {"en": "Everything we have for %s", "es": "Todo lo que tenemos para %s"},
	"nf-all-places":      {"en": "All the places we cover", "es": "Todos los lugares que cubrimos"},
	"nf-search-home":     {"en": "Search from the homepage", "es": "Busque desde la página principal"},
	"nf-why-missing":     {"en": "Why some pages are missing", "es": "Por qué faltan algunas páginas"},
	"nf-why-body": {
		"en": "Every guide on this site is researched, and a person checks every citation before we publish it. We publish nothing without that check. That is why coverage grows one place and one topic at a time. If we do not cover your city yet, your state’s guide and the nationwide guides still apply to you.",
		"es": "Cada guía de este sitio se investiga, y una persona verifica cada cita antes de publicarla. No publicamos nada sin esa verificación. Por eso la cobertura crece lugar por lugar y tema por tema. Si aún no cubrimos su ciudad, la guía de su estado y las guías nacionales igual aplican para usted.",
	},
	"nf-ask-cover": {"en": "Ask us to cover your city →", "es": "Pídanos cubrir su ciudad →"},
}

var esMonths = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// UIString returns the chrome string for a language, falling back to English,
// then to the bare key (ugly on purpose: a typo'd key must be visible).
func UIString(lang, key string) string {
	byLang, ok := uiStrings[key]
	if !ok {
		return key
	}
	if s, ok := byLang[lang]; ok {
		return s
	}
	return byLang["en"]
}

// UIStringf is UIString for strings with fmt verbs.
func UIStringf(lang, key string, args ...any) string {
	return fmt.Sprintf(UIString(lang, key), args...)
}

// UIDate renders a date the way the page's language writes one.
func UIDate(lang string, t time.Time) string {
	if lang == "es" {
		return fmt.Sprintf("%d de %s de %d", t.Day(), esMonths[t.Month()-1], t.Year())
	}
	return t.Format("January 2, 2006")
}

// pageLang reports the language a page renders in, for templates shared
// across page types (the footer, the header search). English-only pages fall
// through to "en" without needing a language field of their own.
func pageLang(v any) string {
	switch p := v.(type) {
	case PlaybookPage:
		if p.Playbook.Language != "" {
			return p.Playbook.Language
		}
	case JurisdictionPage:
		if p.Language != "" {
			return p.Language
		}
	case TopicHubPage:
		if p.Language != "" {
			return p.Language
		}
	case NotFoundPage:
		if p.Language != "" {
			return p.Language
		}
	}
	return "en"
}
