// @vitest-environment node
import { fileURLToPath } from 'node:url'
import { build } from 'vite'
import { expect, test } from 'vitest'

test('the application Vite config applies React Compiler to a component', async () => {
  const root = fileURLToPath(new URL('../', import.meta.url))
  const result = await build({
    root,
    configFile: `${root}/vite.config.ts`,
    logLevel: 'silent',
    build: {
      write: false,
      minify: false,
      lib: { entry: `${root}/tests/CompilerFixture.jsx`, formats: ['es'] },
      rolldownOptions: {
        external: ['react', 'react/jsx-runtime', 'react/compiler-runtime'],
      },
    },
  })
  const chunks = (Array.isArray(result) ? result : [result])
    .flatMap(output => output.output).filter(output => output.type === 'chunk')
  expect(chunks.some(chunk => /from ["']react\/compiler-runtime["']/.test(chunk.code))).toBe(true)
})
