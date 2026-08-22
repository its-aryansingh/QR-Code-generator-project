import type { Metadata } from 'next';
import { QRTypePage } from '@/components/seo/QRTypePage';

export const metadata: Metadata = {
  title: 'Free Event Calendar QR Code Generator | QRit',
  description: 'Create a calendar event QR code. Let attendees instantly add your event, including location and time, to their phone calendars.',
  keywords: ['event qr code', 'calendar qr code generator', 'add to calendar qr', 'vevent qr code'],
  alternates: {
    canonical: 'https://qrit.com/event-qr-code'
  }
};

export default function EventQRPage() {
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "Event QR Code Generator - QRit",
    "operatingSystem": "Web",
    "applicationCategory": "UtilitiesApplication",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "USD"
    }
  };

  const howToSteps = [
    { title: "Enter Event Details", desc: "Type in your event's title, location, and a brief description." },
    { title: "Set Date & Time", desc: "Pick the exact start and end dates and times for your event." },
    { title: "Download", desc: "Get your QR code and print it on invitations, posters, or tickets." }
  ];

  const useCases = [
    { title: "Wedding Invitations", desc: "Make sure guests save the date correctly by letting them scan to add the wedding to their calendar." },
    { title: "Conferences & Seminars", desc: "Print on brochures so attendees can add specific keynote speeches or workshops to their schedules." },
    { title: "Concerts & Shows", desc: "Include on promotional posters so fans won't forget the gig date." },
    { title: "Webinars & Meetings", desc: "Share digitally so participants can seamlessly block their time." }
  ];

  const faqs = [
    { question: "Which calendar apps are supported?", answer: "Event QR codes use the standard iCalendar (VEvent) format, which is supported by Apple Calendar, Google Calendar, Outlook, and most other mobile calendar apps." },
    { question: "What time zone is used?", answer: "The date and time are typically read in the scanner's local time zone unless otherwise specified. Be careful if you are hosting a cross-timezone online event." },
    { question: "Is the end date required?", answer: "No, but adding an end date helps keep the attendee's calendar properly organized." },
    { question: "Can I add an online meeting link?", answer: "Yes, you can put Zoom, Teams, or Google Meet links in the 'Location' or 'Description' fields." },
    { question: "Do these expire?", answer: "The QR code itself never expires, though it obviously becomes less useful once the event date has passed." }
  ];

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <QRTypePage
        type="event"
        heroH1="Free Event QR Code Generator"
        heroSubtitle="Boost attendance by making it incredibly easy for people to add your event to their mobile calendars with a single scan."
        howToSteps={howToSteps}
        useCases={useCases}
        faqs={faqs}
      />
    </>
  );
}
