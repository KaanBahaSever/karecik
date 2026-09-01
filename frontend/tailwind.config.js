/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'Segoe UI', 'sans-serif'],
      },
      colors: {
        // Karecik corporate palette (landing page + dashboard).
        // 600 is the main tone, 700 the hover tone.
        brand: {
          50: '#eff5ff',
          100: '#dbe7fe',
          200: '#bfd5fe',
          300: '#93b8fd',
          400: '#6092fa',
          500: '#3b6ef6',
          600: '#1d4ed8',
          700: '#1e40af',
          800: '#1e3a8a',
          900: '#172554',
        },
      },
      boxShadow: {
        card: '0 1px 2px 0 rgb(0 0 0 / 0.04), 0 1px 3px 0 rgb(0 0 0 / 0.06)',
        panel: '0 4px 16px -2px rgb(0 0 0 / 0.08)',
      },
      maxWidth: {
        content: '1200px',
      },
    },
  },
  plugins: [],
}
