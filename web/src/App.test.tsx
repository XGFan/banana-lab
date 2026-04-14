import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import App from './App'

describe('App', () => {
  it('renders the app shell and sidebar navigation', () => {
    render(
      <MemoryRouter initialEntries={["/txt2img"]}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByTestId('app-shell')).toBeInTheDocument()
    expect(screen.getByTestId('tab-txt2img')).toBeInTheDocument()
  })
})
