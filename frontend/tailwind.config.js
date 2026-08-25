/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'Segoe UI', 'sans-serif'],
      },
      colors: {
        // Karecik kurumsal paleti (landing + panel)
        marka: {
          50: '#f0f9f5',
          100: '#dbf0e5',
          200: '#b9e1cd',
          300: '#8bcbad',
          400: '#57ae88',
          500: '#35916c',
          600: '#1a7f5a',
          700: '#175d44',
          800: '#144a38',
          900: '#123d2f',
        },
      },
      boxShadow: {
        kart: '0 1px 2px 0 rgb(0 0 0 / 0.04), 0 1px 3px 0 rgb(0 0 0 / 0.06)',
        panel: '0 4px 16px -2px rgb(0 0 0 / 0.08)',
      },
      maxWidth: {
        icerik: '1200px',
      },
    },
  },
  plugins: [],
}
