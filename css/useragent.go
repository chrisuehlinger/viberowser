// Package css provides the user agent stylesheet.
// Based on the HTML5 specification default styles.
package css

// UserAgentStylesheet contains the default browser styles.
var UserAgentStylesheet = `
/* Block elements */
html, body, div, article, aside, footer, header, nav, section,
main, figure, figcaption, blockquote, pre, address {
	display: block;
}

/* HTML and Body defaults */
html {
	display: block;
}

body {
	display: block;
	margin: 8px;
}

/* Headings */
h1, h2, h3, h4, h5, h6 {
	display: block;
	font-weight: bold;
}

h1 {
	font-size: 2em;
	margin-top: 0.67em;
	margin-bottom: 0.67em;
}

h2 {
	font-size: 1.5em;
	margin-top: 0.83em;
	margin-bottom: 0.83em;
}

h3 {
	font-size: 1.17em;
	margin-top: 1em;
	margin-bottom: 1em;
}

h4 {
	font-size: 1em;
	margin-top: 1.33em;
	margin-bottom: 1.33em;
}

h5 {
	font-size: 0.83em;
	margin-top: 1.67em;
	margin-bottom: 1.67em;
}

h6 {
	font-size: 0.67em;
	margin-top: 2.33em;
	margin-bottom: 2.33em;
}

/* Paragraphs and text blocks */
p {
	display: block;
	margin-top: 1em;
	margin-bottom: 1em;
}

blockquote {
	display: block;
	margin-top: 1em;
	margin-bottom: 1em;
	margin-left: 40px;
	margin-right: 40px;
}

pre {
	display: block;
	font-family: monospace;
	white-space: pre;
	margin-top: 1em;
	margin-bottom: 1em;
}

/* Lists */
ul, ol {
	display: block;
	margin-top: 1em;
	margin-bottom: 1em;
	padding-left: 40px;
}

ul {
	list-style-type: disc;
}

ol {
	list-style-type: decimal;
}

li {
	display: list-item;
}

dl {
	display: block;
	margin-top: 1em;
	margin-bottom: 1em;
}

dt {
	display: block;
}

dd {
	display: block;
	margin-left: 40px;
}

/* Links */
a:link {
	color: blue;
	text-decoration: underline;
}

a:visited {
	color: purple;
	text-decoration: underline;
}

/* Emphasis and formatting */
strong, b {
	font-weight: bold;
}

em, i, cite, var, dfn {
	font-style: italic;
}

u, ins {
	text-decoration: underline;
}

s, strike, del {
	text-decoration: line-through;
}

small {
	font-size: smaller;
}

big {
	font-size: larger;
}

sub {
	font-size: smaller;
	vertical-align: sub;
}

sup {
	font-size: smaller;
	vertical-align: super;
}

/* Code and keyboard */
code, kbd, samp, tt {
	font-family: monospace;
}

/* Inline elements */
span, a, em, strong, b, i, u, s, sub, sup, small, big,
code, kbd, samp, tt, var, cite, dfn, abbr, mark, q {
	display: inline;
}

/* Abbreviation */
abbr[title] {
	text-decoration: underline dotted;
}

/* Mark */
mark {
	background-color: yellow;
	color: black;
}

/* Horizontal rule */
hr {
	display: block;
	margin-top: 0.5em;
	margin-bottom: 0.5em;
	border-style: inset;
	border-width: 1px;
}

/* Tables */
table {
	display: table;
	border-collapse: separate;
	border-spacing: 2px;
	border-color: gray;
}

caption {
	display: table-caption;
	text-align: center;
}

thead {
	display: table-header-group;
	vertical-align: middle;
}

tbody {
	display: table-row-group;
	vertical-align: middle;
}

tfoot {
	display: table-footer-group;
	vertical-align: middle;
}

tr {
	display: table-row;
	vertical-align: inherit;
}

td, th {
	display: table-cell;
	vertical-align: inherit;
	padding: 1px;
}

th {
	font-weight: bold;
	text-align: center;
}

colgroup {
	display: table-column-group;
}

col {
	display: table-column;
}

/* Forms */
input, button, select, textarea {
	display: inline-block;
}

button {
	text-align: center;
}

fieldset {
	display: block;
	margin-left: 2px;
	margin-right: 2px;
	padding-top: 0.35em;
	padding-bottom: 0.625em;
	padding-left: 0.75em;
	padding-right: 0.75em;
	border: 2px groove;
}

legend {
	display: block;
	padding-left: 2px;
	padding-right: 2px;
}

/* Hidden elements */
head, meta, link, style, script, title, noscript, template {
	display: none;
}

/* Hidden attribute */
[hidden] {
	display: none;
}

/* Images and media */
img {
	display: inline;
}

iframe {
	display: inline;
	border: 2px inset;
}

video, audio {
	display: inline;
}

/* Object and embed */
object, embed {
	display: inline;
}

/* Canvas */
canvas {
	display: inline;
}

/* SVG */
svg {
	display: inline;
}

/* Meter and progress */
meter, progress {
	display: inline-block;
	vertical-align: -0.2em;
}

/* Details and summary */
details {
	display: block;
}

summary {
	display: block;
}

/* Dialog */
dialog {
	display: block;
	position: absolute;
	left: 0;
	right: 0;
	margin: auto;
	border: solid;
	padding: 1em;
	background: white;
	color: black;
}

dialog:not([open]) {
	display: none;
}

/* Output */
output {
	display: inline;
}

/* Ruby */
ruby {
	display: ruby;
}

rt {
	display: ruby-text;
}

rp {
	display: none;
}

/* Bdi/Bdo */
bdi {
	unicode-bidi: isolate;
}

bdo {
	unicode-bidi: bidi-override;
}

/* BR and WBR */
br {
	display: inline;
}

wbr {
	display: inline;
}

/* Address */
address {
	display: block;
	font-style: italic;
}

/* Figure */
figure {
	display: block;
	margin-top: 1em;
	margin-bottom: 1em;
	margin-left: 40px;
	margin-right: 40px;
}

figcaption {
	display: block;
}

/* Time */
time {
	display: inline;
}

/* Data */
data {
	display: inline;
}

/* Sections */
article, aside, nav, section {
	display: block;
}

header, footer {
	display: block;
}

main {
	display: block;
}

/* Hgroup */
hgroup {
	display: block;
}
`

// GetUserAgentStylesheet parses and returns the user agent stylesheet.
func GetUserAgentStylesheet() *Stylesheet {
	parser := NewParser(UserAgentStylesheet)
	return parser.Parse()
}
