import { EditorView } from "@codemirror/view"
import type { Extension } from "@codemirror/state"
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language"
import { tags as t } from "@lezer/highlight"


// VSCode Dark Theme
// Using https://github.com/one-dark/vscode-one-dark-theme/ as reference for the colors
const darkBackground = "#21252b"
const highlightBackground = "#2c313a"
const selection = "#f2c94c22"
const cursor = "#528bff"
const background = "transparent"


export const colors: Record<string, string> = {
    // grey: "#4d5a5e",
    "grey-light": "#6d8086",
    blue: "#519aba",
    green: "#8dc149",
    orange: "#e37933",
    pink: "#f55385",
    purple: "#a074c4",
    red: "#EE6167",
    lightyellow: "#d19a66",
    ignore: "#7494a3",
    white: "#fdf6e3",
    yellow: "#cbcb41",
    chalky: "#e5c07b",
    coral: "#e06c75",
    cyan: "#56b6c2",
    invalid: "#ffffff",
    ivory: "#abb2bf",
    stone: "#7d8799",
    malibu: "#61afef",
    sage: "#98c379",
    whiskey: "#d19a66",
    violet: "#c678dd",
    cursor: "#528bff"
}

export const oneDarkTheme = EditorView.theme({
    "&": {
        color: colors.ivory,
        backgroundColor: colors.background,
        fontSize: "12px",
        fontWeight: "normal",
    },

    ".cm-content": {
        caretColor: cursor
    },

    ".cm-cursor, .cm-dropCursor": { borderLeftColor: cursor },
    "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": { backgroundColor: selection },

    ".cm-panels": { backgroundColor: darkBackground, color: colors.ivory },
    ".cm-panels.cm-panels-top": { borderBottom: "2px solid black" },
    ".cm-panels.cm-panels-bottom": { borderTop: "2px solid black" },

    ".cm-activeLine": { backgroundColor: "#6699ff0b" },
    ".cm-selectionMatch": { backgroundColor: "#aafe661a" },

    "&.cm-focused .cm-matchingBracket, &.cm-focused .cm-nonmatchingBracket": {
        backgroundColor: "#bad0f847"
    },

    ".cm-gutters": {
        backgroundColor: background,
        color: colors.stone,
        border: "none"
    },

    ".cm-activeLineGutter": {
        backgroundColor: highlightBackground
    },

    ".cm-foldPlaceholder": {
        backgroundColor: "transparent",
        border: "none",
        color: "#ddd"
    },

    // ".cm-searchMatch": {
    //   backgroundColor: "#72a1ff59",
    //   outline: "1px solid #457dff"
    // },
    // ".cm-searchMatch.cm-searchMatch-selected": {
    //   backgroundColor: "#6199ff2f"
    // },
    // ".cm-tooltip": {
    //   border: "none",
    //   // backgroundColor: tooltipBackground
    // },
    // ".cm-tooltip .cm-tooltip-arrow::before": {
    // content:'',
    // borderTopColor: "transparent",
    // borderBottomColor: "transparent"
    // },
    // ".cm-tooltip .cm-tooltip-arrow::after": {
    //   content:'',
    //   borderTopColor: tooltipBackground,
    //   borderBottomColor: tooltipBackground
    // },
    // ".cm-lineWrapping": {
    //   wordBreak: "break-all",
    // },
    // ".cm-tooltip-autocomplete": {
    //   "& > ul > li[aria-selected]": {
    //     // backgroundColor: "#000",
    //     color: ivory
    //   }
    // },
}, { dark: true })

/// The highlighting style for code in the One Dark theme.
export const oneDarkHighlightStyle = HighlightStyle.define([
    {
        tag: t.keyword,
        color: colors.violet
    },
    {
        tag: [t.name, t.deleted, t.character, t.propertyName, t.macroName],
        color: colors.coral
    },
    {
        tag: [t.function(t.variableName), t.labelName],
        color: colors.malibu
    },
    {
        tag: [t.color, t.constant(t.name), t.standard(t.name)],
        color: colors.whiskey
    },
    {
        tag: [t.definition(t.name), t.separator],
        color: colors.ivory
    },
    {
        tag: [t.typeName, t.className, t.number, t.changed, t.annotation, t.modifier, t.self, t.namespace],
        color: colors.chalky
    },
    {
        tag: [t.operator, t.operatorKeyword, t.url, t.escape, t.regexp, t.link, t.special(t.string)],
        color: colors.cyan
    },
    {
        tag: [t.meta, t.comment],
        color: colors.stone
    },
    {
        tag: t.strong,
        fontWeight: "bold"
    },
    {
        tag: t.emphasis,
        fontStyle: "italic"
    },
    {
        tag: t.strikethrough,
        textDecoration: "line-through"
    },
    {
        tag: t.link,
        color: colors.stone,
        textDecoration: "underline"
    },
    {
        tag: t.heading,
        fontWeight: "bold",
        color: colors.coral
    },
    {
        tag: [t.atom, t.bool, t.special(t.variableName)],
        color: colors.whiskey
    },
    {
        tag: [t.processingInstruction, t.string, t.literal, t.inserted],
        color: colors.sage
    },
    {
        tag: t.invalid,
        color: colors.invalid
    },
])

/// Extension to enable the One Dark theme (both the editor theme and
/// the highlight style).
export const httpEditorTheme: Extension = [oneDarkTheme, syntaxHighlighting(oneDarkHighlightStyle)]
