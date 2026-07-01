import type { Config } from 'tailwindcss';
import forms from '@tailwindcss/forms';

const config: Config = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: '#1e3a5f',
          50: '#e8eef6',
          100: '#c5d3e7',
          200: '#9fb6d5',
          300: '#7898c3',
          400: '#5b82b5',
          500: '#3d6ca7',
          600: '#33629c',
          700: '#27558e',
          800: '#1e3a5f',
          900: '#122240',
        },
        accent: {
          DEFAULT: '#c9a84c',
          50: '#fdf8ec',
          100: '#f8edcc',
          200: '#f3e1a9',
          300: '#edd585',
          400: '#e8cb69',
          500: '#e3c14d',
          600: '#d9b641',
          700: '#c9a84c',
          800: '#b8942e',
          900: '#9a7a1e',
        },
        neutral: {
          50: '#f8fafc',
          100: '#f1f5f9',
          200: '#e2e8f0',
          300: '#cbd5e1',
          400: '#94a3b8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1e293b',
          900: '#0f172a',
        },
      },
      fontFamily: {
        inter: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [forms],
};

export default config;
