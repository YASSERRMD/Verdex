'use client';

import { useState } from 'react';
import clsx from 'clsx';
import { PlusIcon, Trash2Icon } from 'lucide-react';
import { apiFetch } from '@/lib/api';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { Button } from '@/components/ui/Button';
import type { TimelineEvent, TimelineParty } from '@/types';

export interface PartyTimelineEditorProps {
  parties: TimelineParty[];
  events: TimelineEvent[];
  onPartiesChange: (parties: TimelineParty[]) => void;
  onEventsChange: (events: TimelineEvent[]) => void;
  className?: string;
}

const ROLE_OPTIONS = [
  { value: 'first_party', label: 'First Party' },
  { value: 'second_party', label: 'Second Party' },
  { value: 'third_party', label: 'Third Party' },
] as const;

function newId(prefix: string): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}`;
}

/**
 * UI-only editor for case parties and timeline events, mirroring
 * packages/timeline's Party/Event concepts. Persists changes via a stub
 * API handler; wiring to the real timeline service is a later phase.
 */
export function PartyTimelineEditor({
  parties,
  events,
  onPartiesChange,
  onEventsChange,
  className,
}: PartyTimelineEditorProps) {
  const [savingParty, setSavingParty] = useState(false);
  const [savingEvent, setSavingEvent] = useState(false);
  const [partyError, setPartyError] = useState<string | null>(null);
  const [eventError, setEventError] = useState<string | null>(null);

  const addParty = async () => {
    setPartyError(null);
    setSavingParty(true);
    const party: TimelineParty = {
      id: newId('party'),
      role: 'first_party',
      name: '',
    };
    try {
      await apiFetch('/api/v1/parties', { method: 'POST', body: JSON.stringify(party) });
      onPartiesChange([...parties, party]);
    } catch (err) {
      setPartyError(err instanceof Error ? err.message : 'Failed to add party.');
    } finally {
      setSavingParty(false);
    }
  };

  const updateParty = (id: string, patch: Partial<TimelineParty>) => {
    onPartiesChange(parties.map((p) => (p.id === id ? { ...p, ...patch } : p)));
  };

  const removeParty = (id: string) => {
    onPartiesChange(parties.filter((p) => p.id !== id));
  };

  const addEvent = async () => {
    setEventError(null);
    setSavingEvent(true);
    const event: TimelineEvent = {
      id: newId('event'),
      description: '',
    };
    try {
      await apiFetch('/api/v1/timeline/events', { method: 'POST', body: JSON.stringify(event) });
      onEventsChange([...events, event]);
    } catch (err) {
      setEventError(err instanceof Error ? err.message : 'Failed to add event.');
    } finally {
      setSavingEvent(false);
    }
  };

  const updateEvent = (id: string, patch: Partial<TimelineEvent>) => {
    onEventsChange(events.map((e) => (e.id === id ? { ...e, ...patch } : e)));
  };

  const removeEvent = (id: string) => {
    onEventsChange(events.filter((e) => e.id !== id));
  };

  return (
    <div className={clsx('space-y-8', className)}>
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-neutral-800">Parties</h2>
            <p className="mt-1 text-sm text-neutral-500">Add or edit the parties to this case.</p>
          </div>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            leftIcon={<PlusIcon className="h-4 w-4" />}
            onClick={addParty}
            loading={savingParty}
          >
            Add Party
          </Button>
        </div>

        {partyError && (
          <p role="alert" className="text-sm text-red-600">
            {partyError}
          </p>
        )}

        {parties.length === 0 ? (
          <p className="text-sm text-neutral-400">No parties added yet.</p>
        ) : (
          <ul className="space-y-3">
            {parties.map((party) => (
              <li
                key={party.id}
                data-testid={`party-${party.id}`}
                className="grid grid-cols-1 items-end gap-3 rounded-lg border border-neutral-200 bg-white px-4 py-3 sm:grid-cols-[1fr_1fr_auto]"
              >
                <Input
                  label="Name"
                  value={party.name}
                  onChange={(e) => updateParty(party.id, { name: e.target.value })}
                />
                <Select
                  label="Role"
                  value={party.role}
                  options={[...ROLE_OPTIONS]}
                  onChange={(e) =>
                    updateParty(party.id, { role: e.target.value as TimelineParty['role'] })
                  }
                />
                <button
                  type="button"
                  aria-label={`Remove party ${party.name || party.id}`}
                  onClick={() => removeParty(party.id)}
                  className="rounded p-2 text-neutral-400 hover:bg-neutral-100 hover:text-red-600"
                >
                  <Trash2Icon className="h-4 w-4" aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-neutral-800">Timeline Events</h2>
            <p className="mt-1 text-sm text-neutral-500">
              Add or edit chronological events. Draft events may be adjusted before review.
            </p>
          </div>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            leftIcon={<PlusIcon className="h-4 w-4" />}
            onClick={addEvent}
            loading={savingEvent}
          >
            Add Event
          </Button>
        </div>

        {eventError && (
          <p role="alert" className="text-sm text-red-600">
            {eventError}
          </p>
        )}

        {events.length === 0 ? (
          <p className="text-sm text-neutral-400">No timeline events added yet.</p>
        ) : (
          <ul className="space-y-3">
            {events.map((event) => (
              <li
                key={event.id}
                data-testid={`event-${event.id}`}
                className="grid grid-cols-1 items-end gap-3 rounded-lg border border-neutral-200 bg-white px-4 py-3 sm:grid-cols-[2fr_1fr_auto]"
              >
                <Input
                  label="Description"
                  value={event.description}
                  onChange={(e) => updateEvent(event.id, { description: e.target.value })}
                />
                <Input
                  label="Date"
                  type="date"
                  value={event.occurredAt ?? ''}
                  onChange={(e) => updateEvent(event.id, { occurredAt: e.target.value })}
                />
                <button
                  type="button"
                  aria-label={`Remove event ${event.description || event.id}`}
                  onClick={() => removeEvent(event.id)}
                  className="rounded p-2 text-neutral-400 hover:bg-neutral-100 hover:text-red-600"
                >
                  <Trash2Icon className="h-4 w-4" aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
