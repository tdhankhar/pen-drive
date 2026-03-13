type EventDetailsProps = { event: unknown; blocks?: string[] };
type FormInputFieldsProps = { form: unknown; response?: unknown; setResponse?: (value: unknown) => void };
type OAuthClientDetailsProps = { client: unknown };

function EventDetails(_props: EventDetailsProps) {
  return <div>event details</div>;
}

function FormInputFields(_props: FormInputFieldsProps) {
  return <div>form fields</div>;
}

function OAuthClientDetails(_props: OAuthClientDetailsProps) {
  return <div>oauth client</div>;
}

export function MustCatchConditionalRenderFixture(props: {
  event: { id: string } | null;
  form: { id: string } | null;
  oAuthClient: { id: string } | null;
  selectedSegment: { id: string; type: string } | null;
  segment: { id: string; type: string };
  response: unknown;
  setResponse: (value: unknown) => void;
}) {
  const { event, form, oAuthClient, selectedSegment, segment, response, setResponse } = props;

  return (
    <div>
      {/* Based on cal.com SlotSelectionModalHeader.tsx */}
      {event && <EventDetails event={event} blocks={["DURATION"]} />}

      {/* Based on cal.com TestForm.tsx */}
      {form && <FormInputFields form={form} response={response} setResponse={setResponse} />}

      {/* Based on cal.com managed-users-view.tsx */}
      {oAuthClient && <OAuthClientDetails client={oAuthClient} />}

      {/* Based on cal.com FilterSegmentSelect.tsx */}
      {selectedSegment &&
        selectedSegment.type === segment.type &&
        selectedSegment.id === segment.id && <span>selected</span>}
    </div>
  );
}
