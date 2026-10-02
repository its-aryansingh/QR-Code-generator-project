import type { Metadata } from 'next';
import { QRTypePage } from '@/components/seo/QRTypePage';

export const metadata: Metadata = {
  title: 'Free Email QR Code Generator | QRit',
  description: 'Create an email QR code that automatically drafts a message with pre-filled subject and body. Great for customer feedback and support.',
  keywords: ['email qr code', 'mailto qr code generator', 'scan to email', 'qr code email draft'],
  alternates: {
    canonical: 'https://qrit.com/email-qr-code'
  }
};

export default function EmailQRPage() {
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "Email QR Code Generator - QRit",
    "operatingSystem": "Web",
    "applicationCategory": "CommunicationApplication",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "USD"
    }
  };

  const howToSteps = [
    { title: "Set Recipient", desc: "Enter the destination email address (e.g., support@yourcompany.com)." },
    { title: "Add Subject & Body", desc: "Optionally pre-fill the subject line and the main message content." },
    { title: "Download", desc: "Download the QR code and integrate it into your printed or digital materials." }
  ];

  const useCases = [
    { title: "Customer Feedback", desc: "Place on receipts or tables, drafting an email to 'feedback@...' when scanned." },
    { title: "Event RSVPs", desc: "Include on physical invitations so guests can instantly email their RSVP." },
    { title: "Technical Support", desc: "Put on device labels to let users easily send a pre-formatted help request email." },
    { title: "Sales Inquiries", desc: "Add to real estate signs or car windows to let buyers quickly request more details via email." }
  ];

  const faqs = [
    { question: "How does an Email QR code work?", answer: "It encodes a 'mailto:' link. When scanned, it opens the user's default email app (like Gmail or Apple Mail) and creates a new draft to your address." },
    { question: "Can I leave the subject and body blank?", answer: "Yes, only the recipient email address is required. If you leave the rest blank, the user will compose the message from scratch." },
    { question: "Will it send the email automatically?", answer: "No, it only drafts the email. The user still has to review it and press 'Send', which is a security feature of all smartphones." },
    { question: "Does it work on desktop?", answer: "If scanned with a webcam or clicked (if a link), it will open the default mail client configured on the operating system." },
    { question: "Are these QR codes free?", answer: "Yes, our static email QR codes are completely free and never expire." }
  ];

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <QRTypePage
        type="email"
        heroH1="Free Email QR Code Generator"
        heroSubtitle="Make it effortless for people to email you. Generate a QR code that opens a pre-filled email draft when scanned."
        howToSteps={howToSteps}
        useCases={useCases}
        faqs={faqs}
      />
    </>
  );
}
