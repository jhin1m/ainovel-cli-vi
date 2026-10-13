import js from '@eslint/js'
import globals from 'globals'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'

export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'src/routeTree.gen.ts', 'src/api/types.gen.ts'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: { globals: { ...globals.browser } },
    plugins: { 'react-hooks': reactHooks },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // Nội dung LLM là dữ liệu không tin cậy: cấm chèn HTML thô.
      'no-restricted-syntax': [
        'error',
        {
          selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']",
          message: 'Cấm dangerouslySetInnerHTML: nội dung do LLM sinh ra không được tin cậy.',
        },
        {
          selector: "Identifier[name='dangerouslySetInnerHTML']",
          message: 'Cấm dangerouslySetInnerHTML.',
        },
      ],
    },
  },
  { files: ['*.js', '*.mjs', 'scripts/**', 'vite.config.ts'], languageOptions: { globals: globals.node } },
)
