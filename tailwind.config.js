/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{js,jsx,ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        'boston-blue': '#003366',
        'boston-red': '#CC0000',
      }
    },
  },
  plugins: [],
}
