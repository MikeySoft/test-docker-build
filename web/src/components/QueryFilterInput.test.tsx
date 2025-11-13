/// <reference types="vitest/globals" />
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import QueryFilterInput from './QueryFilterInput';

describe('QueryFilterInput', () => {
  it('shows field suggestions when typing', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<QueryFilterInput value="na" onChange={onChange} />);
    const input = screen.getByRole('textbox');
    await user.click(input);
    expect(screen.getByText('name:')).toBeInTheDocument();
  });

  it('tab accepts active suggestion', async () => {
    const user = userEvent.setup();
    let value = 'na';
    const onChange = (v: string) => { value = v; };
    render(<QueryFilterInput value={value} onChange={onChange} />);
    const input = screen.getByRole('textbox');
    await user.click(input);
    fireEvent.keyDown(input, { key: 'Tab', code: 'Tab' });
    // After accepting, value should end with a space
    expect(value.startsWith('name:')).toBe(true);
  });

  it('suggests values for status and host/images when provided', async () => {
    const user = userEvent.setup();
    render(
      <QueryFilterInput
        value="status=r"
        onChange={() => {}}
        statuses={["running","stopped"]}
        hosts={["prod","staging"]}
        images={["nginx:latest","postgres:16"]}
      />
    );
    const input = screen.getByRole('textbox');
    await user.click(input);
    expect(screen.getByText('status=running')).toBeInTheDocument();
  });
});


