
/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './src/**/*.{html,js,svelte,ts}',
    "./node_modules/cybernetic-ui/dist/**/*.{svelte,html}",
  ],
  plugins: [
    require('cybernetic-ui/tailwindplugin'),
    require("@tailwindcss/forms")
  ],
  safelist: [
    {
      pattern: /(cm)-+/, // 👈  This includes bg of all colors and shades
      // pattern: /(bg|border|text)-+/, // 👈  This includes bg of all colors and shades
    },
  ],
}

