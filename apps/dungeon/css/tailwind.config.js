/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["../server/*.templ"],
  theme: {
    extend: {},
  },
  plugins: [
    require("@tailwindcss/forms")
  ],
}
